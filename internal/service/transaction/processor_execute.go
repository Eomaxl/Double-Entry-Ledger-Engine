package transaction

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/logging"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func (p *PostgresTransactionProcessor) executePostTransaction(ctx context.Context, req PostTransactionRequest) (*domain.Transaction, error) {
	ctx, dbSpan := tracing.StartDatabaseSpan(ctx, "transaction", "transactions")
	defer dbSpan.End()

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.Serializable,
	})
	if err != nil {
		tracing.AddErrorAttributes(dbSpan, err)
		logging.LogError(ctx, p.logger, "begin_transaction", err, nil)
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	ctx, validationSpan := tracing.StartValidationSpan(ctx, "accounts_and_balances")
	if err := p.validateAccountsAndBalancesWithLocking(ctx, tx, req); err != nil {
		tracing.AddErrorAttributes(validationSpan, err)
		logging.LogValidationFailure(ctx, p.logger, "accounts_and_balances", map[string]interface{}{
			"entry_count": len(req.Entries),
		}, err)
		validationSpan.End()
		return nil, err
	}
	validationSpan.End()

	transactionID := uuid.New()
	postedAt := time.Now().UTC()

	dbSpan.SetAttributes(attribute.String("ledger.transaction_id", transactionID.String()))
	tracing.AddTransactionAttributes(dbSpan, transactionID.String(), len(req.Entries), "")

	ctx, insertTxnSpan := tracing.StartDatabaseSpan(ctx, "insert", "transactions",
		attribute.String("ledger.transaction_id", transactionID.String()),
	)
	var settledAt *time.Time
	if req.State == domain.TransactionStateSettled {
		settledAt = &postedAt
	}
	if err := p.ledgerRepo.InsertTransaction(ctx, tx, transactionID, req.IdempotencyKey, req.State, postedAt, settledAt, req.Metadata); err != nil {
		tracing.AddErrorAttributes(insertTxnSpan, err)
		logging.LogError(ctx, p.logger, "insert_transaction", err, map[string]interface{}{
			"transaction_id": transactionID.String(),
		})
		insertTxnSpan.End()
		return nil, fmt.Errorf("failed to insert transaction: %w", err)
	}
	insertTxnSpan.End()

	ctx, insertEntriesSpan := tracing.StartDatabaseSpan(ctx, "insert", "entries",
		attribute.String("ledger.transaction_id", transactionID.String()),
		attribute.Int("ledger.entry_count", len(req.Entries)),
	)
	entries, err := p.ledgerRepo.InsertEntries(ctx, tx, transactionID, entryRequestsToLedgerInputs(req.Entries), postedAt)
	if err != nil {
		tracing.AddErrorAttributes(insertEntriesSpan, err)
		logging.LogError(ctx, p.logger, "insert_entries", err, map[string]interface{}{
			"transaction_id": transactionID.String(),
			"entry_count":    len(req.Entries),
		})
		insertEntriesSpan.End()
		return nil, fmt.Errorf("failed to insert entries: %w", err)
	}
	insertEntriesSpan.End()

	result := &domain.Transaction{
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

	ctx, eventSpan := tracing.StartEventSpan(ctx, "transaction.posted",
		attribute.String("ledger.transaction_id", transactionID.String()),
	)
	if err := p.eventPublisher.PublishTransactionPosted(ctx, result); err != nil {
		tracing.AddErrorAttributes(eventSpan, err)
		logging.LogError(ctx, p.logger, "emit_transaction_posted_event", err, map[string]interface{}{
			"transaction_id": transactionID.String(),
		})
		eventSpan.End()
		p.metrics.EventEmissionErrors.WithLabelValues("transaction.posted").Inc()
		return nil, fmt.Errorf("failed to emit transaction posted event: %w", err)
	}
	eventSpan.End()
	p.metrics.EventEmissionTotal.WithLabelValues("transaction.posted", "success").Inc()

	if err := tx.Commit(ctx); err != nil {
		tracing.AddErrorAttributes(dbSpan, err)
		logging.LogError(ctx, p.logger, "commit_transaction", err, map[string]interface{}{
			"transaction_id": transactionID.String(),
		})
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		if err := p.idempotencyRepo.RecordSuccess(ctx, *req.IdempotencyKey, transactionID, p.idempotencyTTL); err != nil {
			logging.LogError(ctx, p.logger, "record_idempotency_success", err, map[string]interface{}{
				"idempotency_key": *req.IdempotencyKey,
				"transaction_id":  transactionID.String(),
				"note":            "transaction already committed",
			})
		}
	}

	p.logger.Info("transaction posted successfully",
		zap.String("transaction_id", transactionID.String()),
		zap.String("state", string(req.State)))

	return result, nil
}
