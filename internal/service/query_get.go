package service

import (
	"context"
	"fmt"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetTransaction retrieves a transaction by ID with all entries
func (q *PostgresQueryService) GetTransaction(ctx context.Context, txnID string) (*domain.Transaction, error) {
	txnUUID, err := uuid.Parse(txnID)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction ID format: %s", txnID)
	}

	return q.GetTransactionByUUID(ctx, txnUUID)
}

// GetTransactionByUUID retrieves a transaction by typed UUID.
func (q *PostgresQueryService) GetTransactionByUUID(ctx context.Context, transactionID uuid.UUID) (*domain.Transaction, error) {
	transactionIDStr := transactionID.String()
	q.logger.Debug("getting transaction by ID", zap.String("transaction_id", transactionIDStr))

	txn, err := q.ledger.GetTransactionWithEntries(ctx, q.pool, transactionID)
	if err != nil {
		q.logger.Warn("failed to get transaction", zap.Error(err), zap.String("transaction_id", transactionIDStr))
		return nil, err
	}

	q.logger.Debug("transaction retrieved successfully",
		zap.String("transaction_id", transactionIDStr),
		zap.Int("entry_count", len(txn.Entries)))

	return txn, nil
}
