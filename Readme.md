# Double Entry Ledger Engine

## Introduction

The Double entry ledger engine is a high integrity, immutable, multi-currency ledger service that enforces strict double entry accounting principles. It serves as a functional money state engine for financial system including wallets, bank accounts, merchant settlements, and internal accounting. The system guarantees atomic transactions , strong consistency, immutability, auditability, and idempotent operations while supporting high throughput (>= 100K TPS target).

## Glossary

- **DELE**: The Double-Entry Ledger Engine system
- **Account**: A financial account that holds balances in one or more currencies
- **Transaction**: An atomic operation that moves money between accounts following double-entry rules
- **Entry**: A single debit or credit record within a transaction
- **Ledger**: The complete immutable record of all transaction
- **Balance**: The current or historical sum of all entries for an account in a specific currency
- **Pending_State**: A transaction state indicating authorization but not yet settlement
- **Settled_State**: A transaction state indicating final posting to the ledger
- **Reversal**: A compensating transaction that negats a previous transaction
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

The system follows a clean layered architecture:

```
┌─────────────────────────────────────────┐
│         API Layer (HTTP/REST)           │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│        Application Service Layer        │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│          Domain Logic Layer             │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│         Data Access Layer               │
└──────────────────┬──────────────────────┘
                   │
┌──────────────────▼──────────────────────┐
│      Infrastructure Layer               │
│  (PostgreSQL, NATS, Metrics, Logging)   │
└─────────────────────────────────────────┘
```

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
│   └── server/          # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── domain/          # Domain models and business logic
│   ├── repository/      # Data access layer
│   ├── service/         # Application services
│   ├── api/             # HTTP handlers and routing
│   └── infrastructure/  # Database, NATS, logging, metrics
├── pkg/
│   └── types/           # Shared types and utilities
├── migrations/          # Database migration files
├── docker-compose.yml   # Local development services
├── Makefile            # Development commands
└── README.md
```


## Performance

Target performance characteristics:
- **Throughput**: >= 100,000 transactions per second
- **Latency**: Sub-second p99 latency
- **Scalability**: Horizontal scaling via stateless application servers

## Status

🚧 **Under Active Development** 🚧

This project is currently in development. Core infrastructure is in place, and domain logic implementation is in progress.