# Database Integration Tests

This directory contains property-based tests for database operations, including referential integrity enforcement.

## Prerequisites

To run these test, we need :

1. **Docker and Docker compose** installed
2. **PostgreSQL database** running (via docker compose)
3. **Test database** created

## Setup Instructions

### 1. Start PostgreSQL

```bash
docker-compose up -d postgres
```

Wait for PostgreSQL to be ready (check using `docker-compose ps`).

### 2. Create Test Database

```bash
# Connect to PostgreSQL and create the test database
docker exec -it ledger-postgres psql -U postgres -c "CREATE DATABASE ledger_test;"
```

Alternatively, we can use psql directly.

```bash
psql -h localhost -U postgres -c "CREATE DATABASE ledger_test;"
```

### 3. Run the Tests

```bash
# Run all database tests
go test -v ./internal/infrastructure/database

# Run only the referential integrity property test
go test -v -run TestProperty_ReferentialIntegrityEnforcement ./internal/infrastructure/database
```

## Test Configuration

The tests use the following default connection parameters:

- **Host**: localhost
- **Port**: 5432
- **User**: postgres
- **Password**: postgres
- **Database**: ledger_test

You can override these using environment variables:

```bash
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_NAME=ledger_test

go test -v ./internal/infrastructure/database
```

## Property-Based Tests

### Property 49: Referential Integrity Enforcement

This test validates that the database enforces foreign key constraints to maintain referential integrity (Requirement 22.5).

The test verifies that:

1. **Entries cannot reference non-existent transactions** - Attempts to insert entries with invalid transaction_id are rejected
2. **Entries cannot reference non-existent accounts** - Attempts to insert entries with invalid account_id are rejected
3. **Account currencies must reference existing accounts** - Attempts to insert account_currencies with invalid account_id are rejected
4. **Idempotency keys must reference existing transactions** - Attempts to insert idempotency_keys with invalid transaction_id are rejected
5. **Balance snapshots must reference existing accounts** - Attempts to insert balance_snapshots with invalid account_id are rejected
6. **Transaction reversal references must be valid** - Attempts to insert transactions with invalid reverses_transaction_id are rejected

Each property is tested with 100 randomly generated test cases using the gopter property-based testing framework.

## Troubleshooting

### Test Skipped

If the test is skipped with a message like "Database not available for integration test", ensure:

1. Docker is running: `docker ps`
2. PostgreSQL container is running: `docker-compose ps`
3. PostgreSQL is accepting connections: `docker-compose logs postgres`
4. Test database exists: `docker exec -it ledger-postgres psql -U postgres -l`

### Connection Refused

If you see "connection refused" errors:

1. Check if PostgreSQL is running: `docker-compose ps`
2. Check PostgreSQL logs: `docker-compose logs postgres`
3. Verify port 5432 is not in use by another process: `lsof -i :5432`

### Migration Errors

If migrations fail to apply:

1. Check migration files exist: `ls -la migrations/`
2. Verify migration file paths in the test code
3. Check PostgreSQL logs for errors: `docker-compose logs postgres`

## Cleanup

To clean up after testing:

```bash
# Stop and remove containers
docker-compose down

# Remove volumes (WARNING: This deletes all data)
docker-compose down -v
```