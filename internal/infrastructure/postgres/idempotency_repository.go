package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var _ repository.IdempotencyRepository = (*PostgresIdempotencyRepository)(nil)

type PostgresIdempotencyRepository struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

func NewPostgresIdempotencyRepository(pool *pgxpool.Pool, logger *zap.Logger) *PostgresIdempotencyRepository {
	return &PostgresIdempotencyRepository{
		pool:   pool,
		logger: logger,
	}
}

// CheckAndReserve atomically checks if an idempotency key exists and reserves it if not
// Uses INSERT ... ON CONFLICT to ensure atomicity
func (r *PostgresIdempotencyRepository) CheckAndReserve(ctx context.Context, key string, ttl time.Duration) (*domain.IdempotencyRecord, domain.IdempotencyStatus, error) {
	// First, check if the key already exists
	selectSQL := `
		SELECT idempotency_key, transaction_id, created_at, expires_at
		FROM idempotency_keys
		WHERE idempotency_key = $1
	`

	var record domain.IdempotencyRecord
	err := r.pool.QueryRow(ctx, selectSQL, key).Scan(
		&record.IdempotencyKey,
		&record.TransactionID,
		&record.CreatedAt,
		&record.ExpiresAt,
	)

	if err == nil {
		// Key exists - check if it has expired (use UTC for comparison)
		now := time.Now().UTC()
		expiresAt := record.ExpiresAt.UTC()
		if now.After(expiresAt) {
			// Key has expired - delete it and treat as new
			deleteSQL := `DELETE FROM idempotency_keys WHERE idempotency_key = $1`
			_, err = r.pool.Exec(ctx, deleteSQL, key)
			if err != nil {
				r.logger.Warn("failed to delete expired idempotency key",
					zap.Error(err),
					zap.String("key", key))
			}
			r.logger.Debug("idempotency key expired, treating as new",
				zap.String("key", key))
			// Return as new - caller should reserve it after creating transaction
			return &domain.IdempotencyRecord{
				IdempotencyKey: key,
				TransactionID:  uuid.Nil,
				CreatedAt:      time.Now().UTC(),
				ExpiresAt:      time.Now().UTC().Add(ttl),
			}, domain.IdempotencyStatusNew, nil
		}

		// Key exists and is not expired
		r.logger.Debug("idempotency key already exists",
			zap.String("key", key),
			zap.String("transaction_id", record.TransactionID.String()))
		return &record, domain.IdempotencyStatusExists, nil
	}

	if err != pgx.ErrNoRows {
		// Unexpected error
		r.logger.Error("failed to query idempotency key",
			zap.Error(err),
			zap.String("key", key))
		return nil, domain.IdempotencyStatusNew, fmt.Errorf("failed to query idempotency key: %w", err)
	}

	// Key does not exist - return as new
	// The caller will need to call RecordSuccess after creating the transaction
	r.logger.Debug("idempotency key is new",
		zap.String("key", key))

	return &domain.IdempotencyRecord{
		IdempotencyKey: key,
		TransactionID:  uuid.Nil,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(ttl),
	}, domain.IdempotencyStatusNew, nil
}

// RecordSuccess links an idempotency key to a successful transaction
func (r *PostgresIdempotencyRepository) RecordSuccess(ctx context.Context, key string, txnID uuid.UUID, ttl time.Duration) error {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	// Use INSERT ... ON CONFLICT to atomically insert or reject if key exists
	insertSQL := `
		INSERT INTO idempotency_keys (idempotency_key, transaction_id, created_at, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO NOTHING
	`

	result, err := r.pool.Exec(ctx, insertSQL, key, txnID, now, expiresAt)
	if err != nil {
		// Check if this is a foreign key violation (transaction doesn't exist)
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23503" { // foreign_key_violation
				r.logger.Error("transaction does not exist",
					zap.String("key", key),
					zap.String("transaction_id", txnID.String()))
				return fmt.Errorf("transaction does not exist: %s", txnID)
			}
		}

		r.logger.Error("failed to record idempotency success",
			zap.Error(err),
			zap.String("key", key),
			zap.String("transaction_id", txnID.String()))
		return fmt.Errorf("failed to record idempotency success: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// This means the key already exists
		// Check if it's associated with the same transaction
		var existingTxnID uuid.UUID
		selectSQL := `SELECT transaction_id FROM idempotency_keys WHERE idempotency_key = $1`
		err := r.pool.QueryRow(ctx, selectSQL, key).Scan(&existingTxnID)
		if err != nil {
			r.logger.Error("failed to query existing idempotency key",
				zap.Error(err),
				zap.String("key", key))
			return fmt.Errorf("failed to query existing idempotency key: %w", err)
		}

		if existingTxnID != txnID {
			r.logger.Warn("idempotency key already associated with different transaction",
				zap.String("key", key),
				zap.String("existing_transaction_id", existingTxnID.String()),
				zap.String("new_transaction_id", txnID.String()))
			return fmt.Errorf("idempotency key already associated with a different transaction")
		}

		// Same transaction - this is fine (idempotent)
		r.logger.Debug("idempotency key already recorded for same transaction",
			zap.String("key", key),
			zap.String("transaction_id", txnID.String()))
		return nil
	}

	r.logger.Debug("idempotency success recorded",
		zap.String("key", key),
		zap.String("transaction_id", txnID.String()))

	return nil
}

// GetExistingResult retrieves the transaction ID associated with an idempotency key
func (r *PostgresIdempotencyRepository) GetExistingResult(ctx context.Context, key string) (uuid.UUID, error) {
	selectSQL := `
		SELECT transaction_id
		FROM idempotency_keys
		WHERE idempotency_key = $1 AND expires_at > NOW()
	`

	var txnID uuid.UUID
	err := r.pool.QueryRow(ctx, selectSQL, key).Scan(&txnID)

	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, fmt.Errorf("idempotency key not found or expired: %s", key)
		}
		r.logger.Error("failed to get existing result",
			zap.Error(err),
			zap.String("key", key))
		return uuid.Nil, fmt.Errorf("failed to get existing result: %w", err)
	}

	r.logger.Debug("existing result retrieved",
		zap.String("key", key),
		zap.String("transaction_id", txnID.String()))

	return txnID, nil
}
