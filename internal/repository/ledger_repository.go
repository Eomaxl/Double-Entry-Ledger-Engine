package repository

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

// DBQuery is implemented by *pgxpool.Pool and pgx.Tx for read paths.
type DBQuery interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// LedgerEntryInput is persisted as a ledger entry row (IDs assigned at insert time).
type LedgerEntryInput struct {
	AccountID    string
	CurrencyCode string
	Amount       decimal.Decimal
	EntryType    domain.EntryType
	Description  string
}

// LedgerRepository persists transactions and entries (write model / command side).
type LedgerRepository interface {
	InsertTransaction(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, idempotencyKey *string, state domain.TransactionState, postedAt time.Time, settledAt *time.Time, metadata map[string]interface{}) error
	InsertReversalTransaction(ctx context.Context, tx pgx.Tx, reversalTxnID, reversesOriginalID uuid.UUID, idempotencyKey *string, state domain.TransactionState, postedAt time.Time, settledAt *time.Time, metadata map[string]interface{}) error
	InsertEntries(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, entries []LedgerEntryInput, createdAt time.Time) ([]domain.Entry, error)

	GetTransactionWithEntries(ctx context.Context, q DBQuery, transactionID uuid.UUID) (*domain.Transaction, error)
	GetTransactionHeaderForUpdate(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) (*domain.Transaction, error)
	GetTransactionWithEntriesForUpdate(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) (*domain.Transaction, error)

	// FetchEntriesByTransactionIDs loads entries for many transactions (ANY($1)), grouped by transaction UUID.
	FetchEntriesByTransactionIDs(ctx context.Context, q DBQuery, transactionIDs []uuid.UUID) (map[uuid.UUID][]domain.Entry, error)

	UpdateTransactionSettled(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, settledAt time.Time) error
	UpdateTransactionCancelled(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) error
	MarkOriginalTransactionReversed(ctx context.Context, tx pgx.Tx, originalTxnID, reversalTxnID uuid.UUID) error
}
