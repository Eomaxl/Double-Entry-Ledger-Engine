package service

import (
	"context"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
)

func (p *PostgresTransactionProcessor) getTransactionByID(ctx context.Context, transactionID uuid.UUID) (*domain.Transaction, error) {
	return p.ledgerRepo.GetTransactionWithEntries(ctx, p.pool, transactionID)
}
