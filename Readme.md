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
- **Throughput (effective)**: ~825 RPS / QPS (30s run, batch size 5, 24,759 successful requests)
- **Latency p95**: 12.621 ms
- **Latency p99**: 22.436 ms
- **Error rate**: 4 failures out of 24,763 requests (~0.016%)

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
