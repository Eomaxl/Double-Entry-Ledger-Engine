# Performance & Scaling — Double Entry Ledger Engine

## Current Infrastructure

```
Internet
    │
    ▼
EC2 t2.micro (us-east-1c)
  ├── ledger-engine     → :8080
  ├── prometheus        → :9090
  ├── grafana           → :3000
  └── node-exporter     → :9100
    │
    ▼
RDS PostgreSQL db.t3.micro
  └── Single-AZ, 20GB gp2
```

| Component      | Spec                        | Constraint                          |
|----------------|-----------------------------|-------------------------------------|
| EC2            | t2.micro, 1 vCPU, 1GB RAM   | Burstable CPU, exhausts under load  |
| RDS            | db.t3.micro, 2 vCPU, 1GB RAM| Shared CPU, ~90 max connections     |
| DB Pool        | 25 max connections          | Requests queue beyond this          |
| Network        | Up to 1Gbps within VPC      | Not a bottleneck at this scale      |

---

## Current Benchmark Results

| Metric        | Value          |
|---------------|----------------|
| Achieved RPS  | ~50 RPS        |
| Effective TPS | ~250 TPS       |
| p50 latency   | 46ms           |
| p95 latency   | 104ms          |
| p99 latency   | 349ms          |
| Error rate    | 0.03%          |
| Batch size    | 5              |

> TPS = RPS × batch size. Each request submits a batch of 5 double-entry transactions.

---

## Achievable Target on Current Free-Tier Infra

**Target: 2,500–3,000 TPS (~500–600 RPS)**

This is achievable by tuning the existing setup without any infrastructure cost increase:

### Optimisations to reach target

**1. Increase DB connection pool**
```env
DB_MAX_CONNECTIONS=75
DB_MIN_CONNECTIONS=10
```

**2. Run trafficgen at higher concurrency**
```bash
docker compose -f docker-compose.prod.yml run --rm \
  -e TRAFFICGEN_CONCURRENCY=100 \
  -e TRAFFICGEN_RPS=500 \
  -e TRAFFICGEN_BATCH_SIZE=5 \
  -e TRAFFICGEN_DURATION=3m \
  trafficgen
```

**3. Monitor CPU during run**
```bash
top  # Watch for sustained 100% CPU — that is the ceiling
```

### Expected results after tuning

| Metric      | Current  | Target     |
|-------------|----------|------------|
| RPS         | 50       | 500–600    |
| TPS         | 250      | 2,500–3,000|
| p95 latency | 104ms    | < 150ms    |
| p99 latency | 349ms    | < 400ms    |
| Error rate  | < 0.1%   | < 0.1%     |

---

## Scaling to 100,000 TPS

100K TPS requires 20,000 RPS (at batch size 5). This demands a fundamentally different
architecture: horizontal scaling, managed connection pooling, and a high-performance
database tier.

### Target Architecture

```
Internet
    │
    ▼
Route 53 (DNS)
    │
    ▼
Application Load Balancer (ALB)
    │
    ├──────────────────────────────────┐
    ▼                                  ▼
EC2 Auto Scaling Group            EC2 Auto Scaling Group
  c5.2xlarge (AZ-1)                c5.2xlarge (AZ-2)
  ├── ledger-engine                 ├── ledger-engine
  └── node-exporter                 └── node-exporter
    │                                  │
    └──────────────┬───────────────────┘
                   │
                   ▼
            PgBouncer (connection pooler)
            on dedicated EC2
                   │
                   ▼
        Aurora PostgreSQL Cluster
          ├── Writer instance (r6g.2xlarge)
          ├── Reader replica 1 (r6g.xlarge)
          └── Reader replica 2 (r6g.xlarge)
                   │
                   ▼
        ElastiCache Redis (balance cache)
          └── r6g.large, Multi-AZ
```

---

### Why Each Component is Needed

#### Application Load Balancer (ALB)
- Distributes traffic across multiple EC2 instances
- Health checks remove unhealthy instances automatically
- SSL termination at the edge
- Sticky sessions not needed (stateless app)

#### EC2 Auto Scaling Group (c5.2xlarge)
- 8 vCPU, 16GB RAM per instance — handles ~3,000–5,000 RPS each
- Auto-scales from 2 to 10 instances based on CPU/request metrics
- Multi-AZ deployment ensures availability during AZ failure
- Target: 4–6 instances at 100K TPS sustained load

#### PgBouncer (Connection Pooler)
- Aurora supports ~5,000 max connections but each costs RAM
- Go app opens many short-lived connections — expensive without pooling
- PgBouncer pools connections: 1,000 app connections → 100 DB connections
- Transaction-mode pooling ideal for this workload
- Runs on a single `t3.medium` EC2, acts as DB proxy

#### Aurora PostgreSQL (Writer + Read Replicas)
- **Writer** handles all INSERT/UPDATE (transaction writes)
- **Read replicas** handle balance queries and reporting
- Aurora is 3× faster than standard RDS PostgreSQL for write-heavy workloads
- Storage auto-scales from 10GB to 128TB
- Automatic failover < 30 seconds
- `r6g.2xlarge` writer: 8 vCPU, 64GB RAM — handles ~15,000 write TPS

#### ElastiCache Redis (Balance Cache)
- Account balance reads are the hottest query in a ledger
- Redis caches current balances with TTL of 100ms–1s
- Read-through cache: cache hit = <1ms vs 10–20ms DB round trip
- Reduces Aurora read replica load by ~70%
- Multi-AZ with automatic failover

---

### Latency Targets at 100K TPS

| Metric      | Current (t2.micro) | Target (scaled)  |
|-------------|-------------------|------------------|
| p50 latency | 46ms              | < 5ms            |
| p95 latency | 104ms             | < 20ms           |
| p99 latency | 349ms             | < 50ms           |
| Error rate  | 0.03%             | < 0.01%          |

Latency improvements come from:
- PgBouncer eliminating connection overhead
- Redis serving balance reads in < 1ms
- c5.2xlarge having dedicated CPU (no burstable throttling)
- Aurora's optimised write path (log-structured storage)

---

### Reliability at 100K TPS

| Risk                    | Mitigation                                      |
|-------------------------|--------------------------------------------------|
| EC2 instance failure    | Auto Scaling replaces within 60s                |
| AZ outage               | Multi-AZ ALB + ASG spans 2 AZs                  |
| DB writer failure       | Aurora auto-failover to replica < 30s           |
| Connection surge        | PgBouncer queues excess connections gracefully  |
| Cache failure           | Redis Multi-AZ; app falls back to DB on miss    |
| Traffic spike           | ASG scales out in ~3 mins; ALB handles burst    |

---

### Estimated AWS Cost at 100K TPS

| Component            | Service                  | Est. Monthly Cost |
|----------------------|--------------------------|-------------------|
| Load Balancer        | ALB                      | ~$25              |
| App servers (4×)     | c5.2xlarge × 4           | ~$550             |
| Connection pooler    | t3.medium × 1            | ~$30              |
| DB writer            | Aurora r6g.2xlarge       | ~$400             |
| DB read replicas (2×)| Aurora r6g.xlarge × 2    | ~$300             |
| Cache                | ElastiCache r6g.large    | ~$100             |
| Storage              | Aurora 500GB             | ~$60              |
| Data transfer        | ~1TB/month               | ~$90              |
| **Total**            |                          | **~$1,555/month** |

> Cost can be reduced ~40% using Reserved Instances (1-year commitment).

---

### Migration Path from Current to 100K TPS

```
Phase 1 — Current (Free Tier)
  Single EC2 t2.micro + RDS t3.micro
  Target: 2,500–3,000 TPS
  Cost: $0/month

Phase 2 — Vertical Scale ($150/month)
  Upgrade to EC2 c5.xlarge + RDS db.r6g.large
  Add PgBouncer
  Target: 10,000–15,000 TPS

Phase 3 — Horizontal Scale ($500/month)
  ALB + 2× EC2 c5.xlarge Auto Scaling Group
  Aurora PostgreSQL + 1 read replica
  Target: 30,000–50,000 TPS

Phase 4 — Full Scale ($1,500/month)
  ALB + 4-6× EC2 c5.2xlarge
  Aurora + 2 read replicas + Redis cache
  Target: 100,000+ TPS
```

---

*This document describes a cost-optimised path from a free-tier portfolio demo to a
production-grade ledger system capable of 100K TPS with sub-20ms p95 latency.*