package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// OptimisticLockVersion represents a version for optimistic locking
type OptimisticLockVersion struct {
	Version   int64
	UpdatedAt time.Time
}

// CheckOptimisticLockVersion validates that the expected version matches the current version
func (cc *ConcurrencyController) CheckOptimisticLockVersion(ctx context.Context, tx pgx.Tx, tableName, idColumn, id string, expectedVersion OptimisticLockVersion) error {
	query := fmt.Sprintf(`
		SELECT updated_at 
		FROM %s 
		WHERE %s = $1
	`, tableName, idColumn)

	var currentUpdatedAt time.Time
	err := tx.QueryRow(ctx, query, id).Scan(&currentUpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("record not found for optimistic lock check: %s", id)
		}
		return fmt.Errorf("failed to check optimistic lock version: %w", err)
	}

	if !sameTimestamp(currentUpdatedAt, expectedVersion.UpdatedAt) {
		cc.logger.Warn("optimistic lock version mismatch",
			zap.String("table", tableName),
			zap.String("id", id),
			zap.Time("expected_updated_at", expectedVersion.UpdatedAt),
			zap.Time("current_updated_at", currentUpdatedAt))

		return &OptimisticLockError{
			Table:             tableName,
			ID:                id,
			ExpectedUpdatedAt: expectedVersion.UpdatedAt,
			CurrentUpdatedAt:  currentUpdatedAt,
		}
	}

	return nil
}

func sameTimestamp(a, b time.Time) bool {
	return a.Truncate(time.Microsecond).Equal(b.Truncate(time.Microsecond))
}

// OptimisticLockError represents an optimistic locking conflict
type OptimisticLockError struct {
	Table             string
	ID                string
	ExpectedUpdatedAt time.Time
	CurrentUpdatedAt  time.Time
}

// Error returns a human-readable optimistic lock conflict message.
func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock conflict on %s %s: expected updated_at %v, but current is %v",
		e.Table, e.ID, e.ExpectedUpdatedAt, e.CurrentUpdatedAt)
}

// IsOptimisticLockError checks if an error is an optimistic lock error
func IsOptimisticLockError(err error) bool {
	var lockErr *OptimisticLockError
	return errors.As(err, &lockErr)
}
