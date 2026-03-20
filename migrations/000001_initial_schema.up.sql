-- Initial schema for double entry ledger engine
-- Creates core tables with foreign key constraints for referential integrity

-- Accounts table: stores account information
CREATE TABLE accounts (
    account_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_type VARCHAR(50) NOT NULL, -- asset, liability, equity, revenue, expense
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    metadata JSONB
);

CREATE INDEX idx_accounts_created_at ON accounts(created_at);
CREATE INDEX idx_accounts_metadata ON accounts USING GIN(metadata);

-- Account currencies table: defines which currencies an account supports
CREATE TABLE account_currencies (
    account_id UUID NOT NULL REFERENCES accounts(account_id),
    currency_code VARCHAR(3) NOT NULL,
    allow_negative BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, currency_code)
);

CREATE INDEX idx_account_currencies_currency ON account_currencies(currency_code);

-- Transactions table: stores transaction metadata
CREATE TABLE transactions (
    transaction_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    idempotency_key VARCHAR(255) UNIQUE,
    state VARCHAR(20) NOT NULL, -- pending, settled, cancelled
    posted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMP,
    reversed_by_transaction_id UUID REFERENCES transactions(transaction_id),
    reverses_transaction_id UUID REFERENCES transactions(transaction_id),
    metadata JSONB
);

CREATE INDEX idx_transactions_idempotency_key ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_transactions_posted_at ON transactions(posted_at);
CREATE INDEX idx_transactions_state ON transactions(state);
CREATE INDEX idx_transactions_metadata ON transactions USING GIN(metadata);

-- Entries table: stores individual debit/credit entries
CREATE TABLE entries (
    entry_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(transaction_id),
    account_id UUID NOT NULL REFERENCES accounts(account_id),
    currency_code VARCHAR(3) NOT NULL,
    amount NUMERIC(28, 8) NOT NULL CHECK (amount >= 0),
    entry_type VARCHAR(10) NOT NULL, -- debit, credit
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entries_transaction_id ON entries(transaction_id);
CREATE INDEX idx_entries_account_currency ON entries(account_id, currency_code, created_at);
CREATE INDEX idx_entries_account_currency_state ON entries(account_id, currency_code, created_at) 
    INCLUDE (amount, entry_type);

-- Idempotency keys table: tracks idempotency keys with expiration
CREATE TABLE idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES transactions(transaction_id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_idempotency_keys_expires_at ON idempotency_keys(expires_at);

-- Balance snapshots table: optimization for balance queries
CREATE TABLE balance_snapshots (
    snapshot_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(account_id),
    currency_code VARCHAR(3) NOT NULL,
    settled_balance NUMERIC(28, 8) NOT NULL,
    pending_debits NUMERIC(28, 8) NOT NULL DEFAULT 0,
    pending_credits NUMERIC(28, 8) NOT NULL DEFAULT 0,
    entry_count BIGINT NOT NULL,
    snapshot_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, currency_code, snapshot_at)
);

CREATE INDEX idx_balance_snapshots_account_currency ON balance_snapshots(account_id, currency_code, snapshot_at DESC);
