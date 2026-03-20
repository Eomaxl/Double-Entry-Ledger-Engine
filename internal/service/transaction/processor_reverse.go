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

// ReverseTransaction creates a reversal transaction that negates a previous transaction
func (p *PostgresTransactionProcessor) ReverseTransaction(ctx context.Context, originalTransactionID string, req ReversalRequest) (*domain.Transaction, error) {
	originalTxnUUID, err := parseUUIDField("original_transaction_id", originalTransactionID)
	if err != nil {
		return nil, err
	}

	return p.ReverseTransactionUUID(ctx, originalTxnUUID, req)
}

// ReverseTransactionUUID creates a reversal transaction using a typed UUID.
func (p *PostgresTransactionProcessor) ReverseTransactionUUID(ctx context.Context, originalTransactionID uuid.UUID, req ReversalRequest) (*domain.Transaction, error) {
	originalTransactionIDStr := originalTransactionID.String()
	p.logger.Info("reversing transaction",
		zap.String("original_transaction_id", originalTransactionIDStr),
		zap.String("state", string(req.State)))

	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		record, status, err := p.idempotencyRepo.CheckAndReserve(ctx, *req.IdempotencyKey, p.idempotencyTTL)
		if err != nil {
			p.logger.Error("failed to check idempotency",
				zap.Error(err),
				zap.String("idempotency_key", *req.IdempotencyKey))
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}

		if status == domain.IdempotencyStatusExists {
			p.logger.Info("idempotency key already exists, returning existing transaction",
				zap.String("idempotency_key", *req.IdempotencyKey),
				zap.String("transaction_id", record.TransactionID.String()))

			return p.getTransactionByID(ctx, record.TransactionID)
		}
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		p.logger.Error("failed to begin transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	originalTxn, err := p.ledgerRepo.GetTransactionWithEntriesForUpdate(ctx, tx, originalTransactionID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ValidationError{
				Field:   "original_transaction_id",
				Message: fmt.Sprintf("original transaction not found: %s", originalTransactionIDStr),
			}
		}
		p.logger.Warn("failed to get original transaction for reversal",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionIDStr))
		return nil, fmt.Errorf("failed to query original transaction: %w", err)
	}

	if err := p.validateReversalEligibility(originalTxn); err != nil {
		p.logger.Warn("original transaction is not eligible for reversal",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionIDStr))
		return nil, err
	}

	reversalEntries := p.createReversalEntries(originalTxn.Entries, req.Description)

	reversalReq := PostTransactionRequest{
		IdempotencyKey: req.IdempotencyKey,
		State:          req.State,
		Entries:        reversalEntries,
		Metadata:       req.Metadata,
	}

	reversalTxn := p.requestToTransaction(reversalReq)
	if err := p.validator.ValidateTransaction(reversalTxn); err != nil {
		p.logger.Warn("reversal transaction validation failed",
			zap.Error(err))
		return nil, fmt.Errorf("reversal validation failed: %w", err)
	}

	if err := p.validateAccountsAndBalances(ctx, tx, reversalReq); err != nil {
		p.logger.Warn("reversal account validation or balance check failed",
			zap.Error(err))
		return nil, err
	}

	reversalTxnID := uuid.New()
	postedAt := time.Now().UTC()

	var reversalSettledAt *time.Time
	if req.State == domain.TransactionStateSettled {
		reversalSettledAt = &postedAt
	}
	if err := p.ledgerRepo.InsertReversalTransaction(ctx, tx, reversalTxnID, originalTransactionID, req.IdempotencyKey, req.State, postedAt, reversalSettledAt, req.Metadata); err != nil {
		p.logger.Error("failed to insert reversal transaction",
			zap.Error(err),
			zap.String("reversal_transaction_id", reversalTxnID.String()))
		return nil, fmt.Errorf("failed to insert reversal transaction: %w", err)
	}

	entries, err := p.ledgerRepo.InsertEntries(ctx, tx, reversalTxnID, entryRequestsToLedgerInputs(reversalReq.Entries), postedAt)
	if err != nil {
		p.logger.Error("failed to insert reversal entries",
			zap.Error(err),
			zap.String("reversal_transaction_id", reversalTxnID.String()))
		return nil, fmt.Errorf("failed to insert reversal entries: %w", err)
	}

	if err := p.ledgerRepo.MarkOriginalTransactionReversed(ctx, tx, originalTransactionID, reversalTxnID); err != nil {
		p.logger.Error("failed to mark original transaction as reversed",
			zap.Error(err),
			zap.String("original_transaction_id", originalTransactionIDStr),
			zap.String("reversal_transaction_id", reversalTxnID.String()))
		return nil, fmt.Errorf("failed to mark original transaction as reversed: %w", err)
	}

	result := &domain.Transaction{
		TransactionID:         reversalTxnID.String(),
		IdempotencyKey:        req.IdempotencyKey,
		State:                 req.State,
		PostedAt:              postedAt,
		ReversesTransactionID: &originalTransactionIDStr,
		Entries:               entries,
		Metadata:              req.Metadata,
	}

	if req.State == domain.TransactionStateSettled {
		result.SettledAt = &postedAt
	}

	if err := p.eventPublisher.PublishTransactionReversed(ctx, result); err != nil {
		p.logger.Error("failed to emit transaction reversed event",
			zap.Error(err),
			zap.String("reversal_transaction_id", reversalTxnID.String()))
		return nil, fmt.Errorf("failed to emit transaction reversed event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		p.logger.Error("failed to commit reversal transaction",
			zap.Error(err),
			zap.String("reversal_transaction_id", reversalTxnID.String()))
		return nil, fmt.Errorf("failed to commit reversal transaction: %w", err)
	}

	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		if err := p.idempotencyRepo.RecordSuccess(ctx, *req.IdempotencyKey, reversalTxnID, p.idempotencyTTL); err != nil {
			p.logger.Error("failed to record idempotency success for reversal (transaction already committed)",
				zap.Error(err),
				zap.String("idempotency_key", *req.IdempotencyKey),
				zap.String("reversal_transaction_id", reversalTxnID.String()))
		}
	}

	p.logger.Info("reversal transaction posted successfully",
		zap.String("reversal_transaction_id", reversalTxnID.String()),
		zap.String("original_transaction_id", originalTransactionIDStr),
		zap.String("state", string(req.State)))

	return result, nil
}
