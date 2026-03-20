package service

import (
	"context"
	"fmt"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/logging"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// PostTransaction posts a new transaction to the ledger
func (p *PostgresTransactionProcessor) PostTransaction(ctx context.Context, req PostTransactionRequest) (*domain.Transaction, error) {
	ctx, span := tracing.StartTransactionSpan(ctx, "post", "",
		attribute.Int("ledger.entry_count", len(req.Entries)),
		attribute.String("ledger.state", string(req.State)),
	)
	defer span.End()

	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		span.SetAttributes(attribute.String("ledger.idempotency_key", *req.IdempotencyKey))
	}

	logging.LogTransactionSubmission(ctx, p.logger, "",
		func() string {
			if req.IdempotencyKey != nil {
				return *req.IdempotencyKey
			}
			return ""
		}(), len(req.Entries))

	p.metrics.ActiveTransactions.Inc()
	defer p.metrics.ActiveTransactions.Dec()

	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		ctx, idempotencySpan := tracing.StartSpan(ctx, "idempotency.check",
			attribute.String("ledger.idempotency_key", *req.IdempotencyKey),
		)

		record, status, err := p.idempotencyRepo.CheckAndReserve(ctx, *req.IdempotencyKey, p.idempotencyTTL)
		if err != nil {
			tracing.AddErrorAttributes(idempotencySpan, err)
			logging.LogError(ctx, p.logger, "idempotency_check", err, map[string]interface{}{
				"idempotency_key": *req.IdempotencyKey,
			})
			idempotencySpan.End()
			span.End()
			return nil, fmt.Errorf("failed to check idempotency: %w", err)
		}
		idempotencySpan.End()

		if status == domain.IdempotencyStatusExists {
			span.SetAttributes(attribute.Bool("ledger.idempotency_duplicate", true))
			p.logger.Info("idempotency key already exists, returning existing transaction",
				zap.String("idempotency_key", *req.IdempotencyKey),
				zap.String("transaction_id", record.TransactionID.String()))

			return p.getTransactionByID(ctx, record.TransactionID)
		}
	}

	txn := p.requestToTransaction(req)

	ctx, validationSpan := tracing.StartValidationSpan(ctx, "transaction",
		attribute.Int("ledger.entry_count", len(req.Entries)),
	)

	if err := p.validator.ValidateTransaction(txn); err != nil {
		tracing.AddErrorAttributes(validationSpan, err)
		logging.LogValidationFailure(ctx, p.logger, "transaction", map[string]interface{}{
			"entry_count": len(req.Entries),
			"state":       string(req.State),
		}, err)
		validationSpan.End()
		span.End()
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	validationSpan.End()

	var result *domain.Transaction
	retryConfig := DefaultRetryConfig()
	err := p.concurrencyController.ExecuteWithRetry(ctx, retryConfig, func(ctx context.Context) error {
		var err error
		result, err = p.executePostTransaction(ctx, req)
		return err
	})

	if err != nil {
		tracing.AddErrorAttributes(span, err)
		p.metrics.TransactionErrors.WithLabelValues("execution_failed").Inc()
		return nil, err
	}

	span.SetAttributes(attribute.String("ledger.transaction_id", result.TransactionID))
	p.metrics.TransactionTotal.WithLabelValues(string(result.State), "success").Inc()

	return result, nil
}
