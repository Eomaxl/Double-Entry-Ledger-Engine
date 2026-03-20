-- Rollback initial schema
-- Drop tables in reverse order to respect foreign key constraints

DROP TABLE IF EXISTS balance_snapshots;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS account_currencies;
DROP TABLE IF EXISTS accounts;