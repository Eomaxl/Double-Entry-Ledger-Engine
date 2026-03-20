package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ repository.LedgerRepository = (*PostgresLedgerRepository)(nil)

type PostgresLedgerRepository struct{}

func NewPostgresLedgerRepository() *PostgresLedgerRepository {
	return &PostgresLedgerRepository{}
}

func (r *PostgresLedgerRepository) InsertTransaction(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, idempotencyKey *string, state domain.TransactionState, postedAt time.Time, settledAt *time.Time, metadata map[string]interface{}) error {
	var metadataJSON []byte
	var err error
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	const insertSQL = `
		INSERT INTO transactions (transaction_id, idempotency_key, state, posted_at, settled_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(ctx, insertSQL,
		transactionID,
		idempotencyKey,
		state,
		postedAt,
		settledAt,
		metadataJSON,
	)
	return err
}

func (r *PostgresLedgerRepository) InsertReversalTransaction(ctx context.Context, tx pgx.Tx, reversalTxnID, reversesOriginalID uuid.UUID, idempotencyKey *string, state domain.TransactionState, postedAt time.Time, settledAt *time.Time, metadata map[string]interface{}) error {
	var metadataJSON []byte
	var err error
	if metadata != nil {
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal reversal metadata: %w", err)
		}
	}

	const insertSQL = `
		INSERT INTO transactions (transaction_id, idempotency_key, state, posted_at, settled_at, reverses_transaction_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = tx.Exec(ctx, insertSQL,
		reversalTxnID,
		idempotencyKey,
		state,
		postedAt,
		settledAt,
		reversesOriginalID,
		metadataJSON,
	)
	return err
}

func (r *PostgresLedgerRepository) InsertEntries(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, entries []repository.LedgerEntryInput, createdAt time.Time) ([]domain.Entry, error) {
	const insertSQL = `
		INSERT INTO entries (entry_id, transaction_id, account_id, currency_code, amount, entry_type, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	out := make([]domain.Entry, len(entries))
	for i, entryReq := range entries {
		entryID := uuid.New()
		_, err := tx.Exec(ctx, insertSQL,
			entryID,
			transactionID,
			entryReq.AccountID,
			entryReq.CurrencyCode,
			entryReq.Amount,
			entryReq.EntryType,
			entryReq.Description,
			createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to insert entry %d: %w", i, err)
		}
		out[i] = domain.Entry{
			EntryID:       entryID.String(),
			TransactionID: transactionID.String(),
			AccountID:     entryReq.AccountID,
			CurrencyCode:  entryReq.CurrencyCode,
			Amount:        entryReq.Amount,
			EntryType:     entryReq.EntryType,
			Description:   entryReq.Description,
			CreatedAt:     createdAt,
		}
	}
	return out, nil
}

func (r *PostgresLedgerRepository) GetTransactionWithEntries(ctx context.Context, q repository.DBQuery, transactionID uuid.UUID) (*domain.Transaction, error) {
	const queryTxnSQL = `
		SELECT transaction_id, idempotency_key, state, posted_at, settled_at,
		       reversed_by_transaction_id, reverses_transaction_id, metadata
		FROM transactions
		WHERE transaction_id = $1
	`

	txn, err := scanFullTransactionHeader(q.QueryRow(ctx, queryTxnSQL, transactionID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("transaction not found: %s", transactionID)
		}
		return nil, err
	}

	entries, err := queryEntriesForTransaction(ctx, q, transactionID)
	if err != nil {
		return nil, err
	}
	txn.Entries = entries
	return txn, nil
}

func (r *PostgresLedgerRepository) GetTransactionHeaderForUpdate(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) (*domain.Transaction, error) {
	const querySQL = `
		SELECT transaction_id, idempotency_key, state, posted_at, settled_at,
		       reversed_by_transaction_id, reverses_transaction_id, metadata
		FROM transactions
		WHERE transaction_id = $1
		FOR UPDATE
	`

	return scanFullTransactionHeader(tx.QueryRow(ctx, querySQL, transactionID))
}

func (r *PostgresLedgerRepository) GetTransactionWithEntriesForUpdate(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) (*domain.Transaction, error) {
	const queryTxnSQL = `
		SELECT transaction_id, idempotency_key, state, posted_at, settled_at,
		       reversed_by_transaction_id, reverses_transaction_id, metadata
		FROM transactions
		WHERE transaction_id = $1
		FOR UPDATE
	`

	txn, err := scanFullTransactionHeader(tx.QueryRow(ctx, queryTxnSQL, transactionID))
	if err != nil {
		return nil, err
	}

	entries, err := queryEntriesForTransaction(ctx, tx, transactionID)
	if err != nil {
		return nil, err
	}
	txn.Entries = entries
	return txn, nil
}

func (r *PostgresLedgerRepository) UpdateTransactionSettled(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID, settledAt time.Time) error {
	const updateSQL = `
		UPDATE transactions
		SET state = $1, settled_at = $2
		WHERE transaction_id = $3
	`
	_, err := tx.Exec(ctx, updateSQL, domain.TransactionStateSettled, settledAt, transactionID)
	return err
}

func (r *PostgresLedgerRepository) UpdateTransactionCancelled(ctx context.Context, tx pgx.Tx, transactionID uuid.UUID) error {
	const updateSQL = `
		UPDATE transactions
		SET state = $1
		WHERE transaction_id = $2
	`
	_, err := tx.Exec(ctx, updateSQL, domain.TransactionStateCancelled, transactionID)
	return err
}

func (r *PostgresLedgerRepository) MarkOriginalTransactionReversed(ctx context.Context, tx pgx.Tx, originalTxnID, reversalTxnID uuid.UUID) error {
	const updateSQL = `
		UPDATE transactions
		SET reversed_by_transaction_id = $1
		WHERE transaction_id = $2
	`
	_, err := tx.Exec(ctx, updateSQL, reversalTxnID, originalTxnID)
	return err
}

func scanFullTransactionHeader(row pgx.Row) (*domain.Transaction, error) {
	txn, idempotencyKey, settledAt, reversedBy, reverses, metadataJSON, err := scanTransactionHeader(row)
	if err != nil {
		return nil, err
	}
	txn.IdempotencyKey = idempotencyKey
	txn.SettledAt = settledAt
	txn.ReversedByTransactionID = reversedBy
	txn.ReversesTransactionID = reverses
	if err := unmarshalMetadata(metadataJSON, &txn.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return txn, nil
}

// ScanTransactionFromRow reads one transaction header row (same column layout as list/detail SQL).
func ScanTransactionFromRow(row pgx.Row) (*domain.Transaction, error) {
	return scanFullTransactionHeader(row)
}

func scanTransactionHeader(row pgx.Row) (*domain.Transaction, *string, *time.Time, *string, *string, []byte, error) {
	var txn domain.Transaction
	var idempotencyKey, reversedBy, reverses *string
	var settledAt *time.Time
	var metadataJSON []byte

	err := row.Scan(
		&txn.TransactionID,
		&idempotencyKey,
		&txn.State,
		&txn.PostedAt,
		&settledAt,
		&reversedBy,
		&reverses,
		&metadataJSON,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil, nil, nil, nil, nil, err
		}
		return nil, nil, nil, nil, nil, nil, fmt.Errorf("failed to query transaction: %w", err)
	}
	return &txn, idempotencyKey, settledAt, reversedBy, reverses, metadataJSON, nil
}

func unmarshalMetadata(metadataJSON []byte, dest *map[string]interface{}) error {
	if len(metadataJSON) == 0 {
		return nil
	}
	return json.Unmarshal(metadataJSON, dest)
}

func queryEntriesForTransaction(ctx context.Context, q repository.DBQuery, transactionID uuid.UUID) ([]domain.Entry, error) {
	const queryEntriesSQL = `
		SELECT entry_id, transaction_id, account_id, currency_code, amount, entry_type, description, created_at
		FROM entries
		WHERE transaction_id = $1
		ORDER BY created_at
	`

	rows, err := q.Query(ctx, queryEntriesSQL, transactionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.Entry, 0)
	for rows.Next() {
		var entry domain.Entry
		if err := rows.Scan(
			&entry.EntryID,
			&entry.TransactionID,
			&entry.AccountID,
			&entry.CurrencyCode,
			&entry.Amount,
			&entry.EntryType,
			&entry.Description,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entries: %w", err)
	}
	return entries, nil
}

func (r *PostgresLedgerRepository) FetchEntriesByTransactionIDs(ctx context.Context, q repository.DBQuery, transactionIDs []uuid.UUID) (map[uuid.UUID][]domain.Entry, error) {
	if len(transactionIDs) == 0 {
		return map[uuid.UUID][]domain.Entry{}, nil
	}

	const query = `
		SELECT entry_id, transaction_id, account_id, currency_code, amount, entry_type, description, created_at
		FROM entries
		WHERE transaction_id = ANY($1)
		ORDER BY transaction_id, created_at
	`

	rows, err := q.Query(ctx, query, transactionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries for transactions: %w", err)
	}
	defer rows.Close()

	entriesMap := make(map[uuid.UUID][]domain.Entry)
	for rows.Next() {
		var entry domain.Entry
		var transactionID uuid.UUID

		if err := rows.Scan(
			&entry.EntryID,
			&transactionID,
			&entry.AccountID,
			&entry.CurrencyCode,
			&entry.Amount,
			&entry.EntryType,
			&entry.Description,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan entry row: %w", err)
		}

		entry.TransactionID = transactionID.String()
		entriesMap[transactionID] = append(entriesMap[transactionID], entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entry rows: %w", err)
	}
	return entriesMap, nil
}
