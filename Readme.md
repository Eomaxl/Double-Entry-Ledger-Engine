# Double Entry Ledger Engine

## Introduction

The Double Entry Ledger Engine is a high-integrity, immutable, multi-currency ledger service that enforces strict double-entry accounting principles. It serves as a functional money-state engine for financial systems, including wallets, bank accounts, merchant settlements, and internal accounting. The system guarantees atomic transactions, strong consistency, immutability, auditability, and idempotent operations while supporting high throughput (>= 100K TPS target).

## Glossary

- **DELE**: The Double-Entry Ledger Engine system
- **Account**: A financial account that holds balances in one or more currencies
- **Transaction**: An atomic operation that moves money between accounts following double-entry rules
- **Entry**: A single debit or credit record within a transaction
- **Ledger**: The complete immutable record of all transactions
- **Balance**: The current or historical sum of all entries for an account in a specific currency
- **Pending_State**: A transaction state indicating authorization but not yet settlement
- **Settled_State**: A transaction state indicating final posting to the ledger
- **Reversal**: A compensating transaction that negates a previous transaction
- **Idempotency_Key**: A unique identifier ensuring duplicate transaction submissions produce identical results
- **Currency_Code**: An ISO 4217 three-letter currency identifier
- ***Transaction_Boundary**: The scope within which consistency guarantees apply
- **Ledger_Event**: An immutable notification emitted when ledger state changes
- **Negative_Balance_Policy**: A configurable rule determining whether an account may have negative balance

## Features

- **Double-Entry Accounting**: Enforces strict double-entry rules (debits = credits)
- **Double-Entry Accounting**: Enforces strict double-entry rules (debits = credits)
- **Multi-Currency Support**: Handle multiple currencies with independent balance tracking
- **Immutability**: Append-only ledger with complete audit trail
- **Idempotency**: Safe retry handling with idempotency keys
- **High Performance**: Optimized for >= 100K TPS throughput
- **Strong Consistency**: ACID guarantees with PostgreSQL
- **Event Streaming**: Real-time event emission via NATS JetStream
- **Observability**: Structured logging (zap), metrics (Prometheus), and tracing (OpenTelemetry)

## Architecture

The system currently follows a layered Go architecture with explicit composition and adapter boundaries:

```
┌──────────────────────────────────────────────┐
│              cmd/server/main.go              │
│        Thin entrypoint and process boot      │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│                internal/app                  │
│    Dependency wiring, startup, shutdown      │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│                internal/api                  │
│   Gin router, handlers, middleware, errors   │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│              internal/service                │
│   Feature subpackages: transaction, balance, │
│   query (business orchestration by feature)  │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│              internal/domain                 │
│    Core models, validation rules, balances   │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│            internal/repository               │
│         Repository contracts/interfaces      │
└──────────────────────┬───────────────────────┘
                       │
┌──────────────────────▼───────────────────────┐
│           internal/infrastructure            │
│ PostgreSQL, logging, tracing, metrics,       │
│ and event publisher adapters                 │
└──────────────────────────────────────────────┘
```

Current runtime notes:

- HTTP transport is implemented with Gin under `internal/api`.
- Application startup and graceful shutdown are coordinated in `internal/app`.
- Service-layer features are split into focused subpackages:
  - `internal/service/transaction`: transaction posting, settle/cancel/reverse, concurrency controls
  - `internal/service/balance`: current/historical/multi-currency balance computation
  - `internal/service/query`: transaction lookup, listing, and account statements
- PostgreSQL is the primary persistence adapter.
- Event publishing is explicitly optional. When disabled, the application uses a no-op publisher; when enabled, a concrete event adapter must be supplied.
- Liveness and readiness endpoints are exposed separately from the general health endpoint.

## Technology Stack

- **Language**: Go 1.21+
- **Web Framework**: Gin
- **Database**: PostgreSQL 15+ with pgx driver
- **Event Streaming**: NATS JetStream
- **Migration Tool**: golang-migrate
- **Decimal Arithmetic**: shopspring/decimal
- **Testing**: testify, gopter (property-based testing)
- **Observability**: zap (logging), Prometheus (metrics), OpenTelemetry (tracing)

## Project Structure

```
.
├── cmd/
│   └── server/              # Application entry point
├── internal/
│   ├── app/                 # Application bootstrap and dependency wiring
│   ├── api/                 # HTTP router, handlers, middleware, API errors
│   ├── config/              # Configuration loading and validation
│   ├── domain/              # Core business models and validation rules
│   ├── repository/          # Repository contracts/interfaces
│   ├── service/             # Service package root and feature packages
│   │   ├── transaction/     # Transaction business flows and concurrency control
│   │   ├── balance/         # Balance computation workflows
│   │   └── query/           # Read/query workflows (transaction and statements)
│   └── infrastructure/
│       ├── database/        # PostgreSQL connection and migrations
│       ├── events/          # Event publisher interfaces/adapters
│       ├── logging/         # Logger initialization and structured logging helpers
│       ├── metrics/         # Prometheus metrics
│       ├── postgres/        # PostgreSQL repository implementations
│       └── tracing/         # OpenTelemetry tracing setup
├── migrations/              # Database migration files
├── docker-compose.yml       # Local development services
├── go.mod
├── go.sum
└── Readme.md
```


## Performance

Current benchmark characteristics (local Docker setup):
- **Max stable request throughput (RPS/QPS)**: ~905.05 (18,101 successes in 20s at target 1200 RPS)
- **Max stable transaction throughput (TPS)**: ~4,525.25 (RPS x batch size 5)
- **Latency p95 at max stable step**: 31.164 ms
- **Latency p99 at max stable step**: 50.803 ms
- **Error rate at max stable step**: 6 failures out of 18,107 requests (~0.033%)
- **Benchmark artifact**: `benchmarks/ramp_summary_20260320_131433.csv`

## AWS Free-Tier Portfolio Deployment

This project includes a recruiter-focused deployment path for a single-node, zero-minimal-cost demo on AWS.

### Deployment objective

- Standalone public demo endpoint on one EC2 free-tier instance
- Observability included (Prometheus + Grafana) for performance proof
- Repeatable synthetic benchmark to show max stable RPS/TPS

### Free-tier cost controls

- Use one `t2.micro`/`t3.micro` EC2 instance only (as per your account free-tier eligibility)
- Use one `gp3` EBS volume (20 GB)
- Avoid paid services (ALB, NAT Gateway, RDS)
- Configure AWS Budget alerts (`$0.01`, `$1`, `$5`)
- Restrict security group inbound:
  - SSH `22` from your IP only
  - App `8080` public for demo
  - Grafana `3000` optional (or keep private)

### EC2 bootstrap and deploy steps

1. SSH into EC2 and install Docker:
   ```bash
   chmod +x scripts/ec2-bootstrap.sh
   sudo ./scripts/ec2-bootstrap.sh
   ```
2. Clone repo and prepare environment:
   ```bash
   git clone https://github.com/<your-org-or-user>/<your-repo>.git
   cd <your-repo>
   cp .env.production.example .env
   ```
3. Edit `.env` with secure values, then deploy:
   ```bash
   chmod +x scripts/deploy-portfolio.sh
   ./scripts/deploy-portfolio.sh
   ```

### Observability (free-tier self-hosted)

Start observability profile:

```bash
docker compose --profile observability up -d prometheus grafana node-exporter
```

- Prometheus: `http://<EC2_PUBLIC_IP>:9090`
- Grafana: `http://<EC2_PUBLIC_IP>:3000`
- Provisioned dashboard: `monitoring/grafana/dashboards/portfolio-overview.json`

### Max-throughput benchmark command

Run the built-in ramp benchmark:

```bash
chmod +x scripts/benchmark-ramp.sh
./scripts/benchmark-ramp.sh
```

This runs target steps `200 -> 400 -> 600 -> 800 -> 1000 -> 1200` RPS and writes:
- per-step result CSV files in `benchmarks/`
- summary file (latest: `benchmarks/ramp_summary_20260320_131433.csv`)

### Latest measured ramp snapshot

| Target RPS | Success (20s) | Effective RPS | Effective TPS (batch=5) | p95 (ms) | p99 (ms) | Failures |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 200 | 3,999 | 199.95 | 999.75 | 5.747 | 33.946 | 0 |
| 400 | 7,932 | 396.60 | 1,983.00 | 8.962 | 34.348 | 0 |
| 600 | 11,361 | 568.05 | 2,840.25 | 7.993 | 24.492 | 1 |
| 800 | 14,443 | 722.15 | 3,610.75 | 18.546 | 42.900 | 1 |
| 1000 | 15,768 | 788.40 | 3,942.00 | 28.600 | 64.206 | 5 |
| 1200 | 18,101 | 905.05 | 4,525.25 | 31.164 | 50.803 | 6 |

For this workload, QPS equals request throughput because each API call is one batch-post request unit.

## Synthetic Traffic Generator

The repository includes a built-in synthetic traffic generator at `cmd/trafficgen` for controlled load tests against the running API.

### Current setup (local Docker)

Based on `docker-compose.yml`, the current benchmark setup is:
- **App service**: `ledger-engine` (Go service, exposed on `:8080`)
- **Database**: `postgres:15-alpine`
- **Messaging**: `nats:2.10-alpine` (currently disabled in app via `NATS_ENABLED=false`)
- **Traffic profile service**: `trafficgen` (Go 1.25 Alpine)
- **Trafficgen defaults**:
  - `TRAFFICGEN_DURATION=30s`
  - `TRAFFICGEN_CONCURRENCY=20`
  - `TRAFFICGEN_RPS=1000` (target request rate)
  - `TRAFFICGEN_BATCH_SIZE=5`
  - `TRAFFICGEN_ACCOUNTS=100`

### How to run

1. Start core services:
   ```bash
   docker compose up -d postgres nats ledger-engine
   ```
2. Run synthetic traffic from Docker Compose profile:
   ```bash
   docker compose --profile trafficgen run --rm trafficgen
   ```
3. Optional local run (without profile container):
   ```bash
   go run ./cmd/trafficgen
   ```
4. Read generated metrics from:
   - `trafficgen_results.csv`
   - `trafficgen_results_local.csv`

### Current measured throughput and latency

Latest measured snapshots from checked-in result files:

| Scenario                                         | Duration | Requests | Success | Fail | Effective RPS | Effective QPS | p95 Latency | p99 Latency |
| ------------------------------------------------ | -------- | -------: | ------: | ---: | -------------:| ------------: | -----------:| ----------: |
| Local short run (`trafficgen_results_local.csv`) | 5.003s   | 999      | 999     | 0    | ~199.68       | ~199.68       | 5.699 ms    | 33.572 ms   |
| Docker profile run (`trafficgen_results.csv`)    | 30.003s  | 24,763   | 24,759  | 4    | ~825.35       | ~825.35       | 12.621 ms   | 22.436 ms   |

Notes:
- For this workload, **QPS is treated equal to request throughput** (each generated API request is a transaction-post workload unit).
- The configured target (`TRAFFICGEN_RPS=1000`) is higher than the currently observed steady throughput (~825 RPS), which indicates the stack is near current capacity under this profile.

## Status

🚧 **Under Active Development** 🚧

This project is currently in development. Core infrastructure is in place, and domain logic implementation is in progress.
