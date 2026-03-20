package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// PostBatch posts multiple transactions atomically as a single batch
func (p *PostgresTransactionProcessor) PostBatch(ctx context.Context, reqs []PostTransactionRequest) ([]domain.Transaction, error) {
	p.logger.Info("posting transaction batch",
		zap.Int("batch_size", len(reqs)))

	if len(reqs) == 0 {
		return nil, domain.ValidationError{
			Field:   "batch",
			Message: "batch cannot be empty",
		}
	}
	if len(reqs) > 1000 {
		return nil, domain.ValidationError{
			Field:   "batch",
			Message: fmt.Sprintf("batch size %d exceeds maximum of 1000 transactions", len(reqs)),
		}
	}

	for i, req := range reqs {
		if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
			record, status, err := p.idempotencyRepo.CheckAndReserve(ctx, *req.IdempotencyKey, p.idempotencyTTL)
			if err != nil {
				p.logger.Error("failed to check idempotency in batch",
					zap.Error(err),
					zap.String("idempotency_key", *req.IdempotencyKey),
					zap.Int("batch_index", i))
				return nil, fmt.Errorf("failed to check idempotency for transaction %d: %w", i, err)
			}

			if status == domain.IdempotencyStatusExists {
				return nil, domain.ValidationError{
					Field:   "idempotency_key",
					Message: fmt.Sprintf("transaction %d: idempotency key %s already exists with transaction %s", i, *req.IdempotencyKey, record.TransactionID.String()),
				}
			}
		}

		txn := p.requestToTransaction(req)
		if err := p.validator.ValidateTransaction(txn); err != nil {
			p.logger.Warn("batch transaction validation failed",
				zap.Error(err),
				zap.Int("batch_index", i))
			return nil, fmt.Errorf("validation failed for transaction %d: %w", i, err)
		}
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		p.logger.Error("failed to begin batch transaction", zap.Error(err))
		return nil, fmt.Errorf("failed to begin batch transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for i, req := range reqs {
		if err := p.validateAccountsAndBalances(ctx, tx, req); err != nil {
			p.logger.Warn("batch account validation or balance check failed",
				zap.Error(err),
				zap.Int("batch_index", i))
			return nil, fmt.Errorf("validation failed for transaction %d: %w", i, err)
		}
	}

	baseTime := time.Now().UTC()
	transactionIDs := make([]uuid.UUID, len(reqs))
	for i := range reqs {
		transactionIDs[i] = uuid.New()
	}

	results := make([]domain.Transaction, len(reqs))
	for i, req := range reqs {
		transactionID := transactionIDs[i]
		postedAt := baseTime.Add(time.Duration(i) * time.Microsecond)

		var settledAt *time.Time
		if req.State == domain.TransactionStateSettled {
			t := postedAt
			settledAt = &t
		}
		if err := p.ledgerRepo.InsertTransaction(ctx, tx, transactionID, req.IdempotencyKey, req.State, postedAt, settledAt, req.Metadata); err != nil {
			p.logger.Error("failed to insert batch transaction",
				zap.Error(err),
				zap.String("transaction_id", transactionID.String()),
				zap.Int("batch_index", i))
			return nil, fmt.Errorf("failed to insert transaction %d: %w", i, err)
		}

		entries, err := p.ledgerRepo.InsertEntries(ctx, tx, transactionID, entryRequestsToLedgerInputs(req.Entries), postedAt)
		if err != nil {
			p.logger.Error("failed to insert batch transaction entries",
				zap.Error(err),
				zap.String("transaction_id", transactionID.String()),
				zap.Int("batch_index", i))
			return nil, fmt.Errorf("failed to insert entries for transaction %d: %w", i, err)
		}

		result := domain.Transaction{
			TransactionID:  transactionID.String(),
			IdempotencyKey: req.IdempotencyKey,
			State:          req.State,
			PostedAt:       postedAt,
			Entries:        entries,
			Metadata:       req.Metadata,
		}

		if req.State == domain.TransactionStateSettled {
			result.SettledAt = &postedAt
		}

		results[i] = result
	}

	for i, result := range results {
		if err := p.eventPublisher.PublishTransactionPosted(ctx, &result); err != nil {
			p.logger.Error("failed to emit batch transaction posted event",
				zap.Error(err),
				zap.String("transaction_id", result.TransactionID),
				zap.Int("batch_index", i))
			return nil, fmt.Errorf("failed to emit event for transaction %d: %w", i, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		p.logger.Error("failed to commit batch transaction",
			zap.Error(err),
			zap.Int("batch_size", len(reqs)))
		return nil, fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	for i, req := range reqs {
		if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
			if err := p.idempotencyRepo.RecordSuccess(ctx, *req.IdempotencyKey, transactionIDs[i], p.idempotencyTTL); err != nil {
				p.logger.Error("failed to record idempotency success for batch transaction (transactions already committed)",
					zap.Error(err),
					zap.String("idempotency_key", *req.IdempotencyKey),
					zap.String("transaction_id", transactionIDs[i].String()),
					zap.Int("batch_index", i))
			}
		}
	}

	p.logger.Info("batch transactions posted successfully",
		zap.Int("batch_size", len(reqs)))

	return results, nil
}
