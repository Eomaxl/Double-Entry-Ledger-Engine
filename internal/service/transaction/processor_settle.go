package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// SettlePendingTransaction settles a pending transaction by transitioning it to settled state
func (p *PostgresTransactionProcessor) SettlePendingTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	txnUUID, err := parseUUIDField("transaction_id", transactionID)
	if err != nil {
		return nil, err
	}

	return p.SettlePendingTransactionUUID(ctx, txnUUID)
}

// SettlePendingTransactionUUID settles a pending transaction using a typed UUID.
func (p *PostgresTransactionProcessor) SettlePendingTransactionUUID(ctx context.Context, transactionID uuid.UUID) (*domain.Transaction, error) {
	transactionIDStr := transactionID.String()
	p.logger.Info("settling pending transaction",
		zap.String("transaction_id", transactionIDStr))

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		p.logger.Error("failed to begin transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txn, err := p.ledgerRepo.GetTransactionHeaderForUpdate(ctx, tx, transactionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ValidationError{
				Field:   "transaction_id",
				Message: fmt.Sprintf("transaction not found: %s", transactionIDStr),
			}
		}
		return nil, fmt.Errorf("failed to query transaction: %w", err)
	}

	if txn.State != domain.TransactionStatePending {
		return nil, domain.ValidationError{
			Field:   "state",
			Message: fmt.Sprintf("transaction %s is not in pending state (current state: %s)", transactionIDStr, txn.State),
		}
	}

	settledAtTime := time.Now().UTC()
	if err := p.ledgerRepo.UpdateTransactionSettled(ctx, tx, transactionID, settledAtTime); err != nil {
		p.logger.Error("failed to update transaction state",
			zap.Error(err),
			zap.String("transaction_id", transactionIDStr))
		return nil, fmt.Errorf("failed to update transaction state: %w", err)
	}

	updatedTxn, err := p.ledgerRepo.GetTransactionWithEntries(ctx, tx, transactionID)
	if err != nil {
		p.logger.Error("failed to get updated transaction for event emission",
			zap.Error(err),
			zap.String("transaction_id", transactionIDStr))
		return nil, fmt.Errorf("failed to get updated transaction: %w", err)
	}

	if err := p.eventPublisher.PublishTransactionSettled(ctx, updatedTxn); err != nil {
		p.logger.Error("failed to emit transaction settled event",
			zap.Error(err),
			zap.String("transaction_id", transactionIDStr))
		return nil, fmt.Errorf("failed to emit transaction settled event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		p.logger.Error("failed to commit transaction",
			zap.Error(err),
			zap.String("transaction_id", transactionIDStr))
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	p.logger.Info("transaction settled successfully",
		zap.String("transaction_id", transactionIDStr))

	return updatedTxn, nil
}
