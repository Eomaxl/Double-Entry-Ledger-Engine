package repository

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AccountRepository interface {
	CreateAccount(ctx context.Context, req domain.CreateAccountRequest) (*domain.Account, error)
	GetAccount(ctx context.Context, accountID uuid.UUID) (*domain.Account, error)
	GetAccountInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID) (*domain.Account, error)
	ListAccounts(ctx context.Context, filter domain.AccountFilter) ([]domain.Account, error)
	UpdateAccountMetadata(ctx context.Context, accountID uuid.UUID, metadata map[string]interface{}) error
}

type IdempotencyRepository interface {
	CheckAndReserve(ctx context.Context, key string, ttl time.Duration) (*domain.IdempotencyRecord, domain.IdempotencyStatus, error)
	RecordSuccess(ctx context.Context, key string, txnID uuid.UUID, ttl time.Duration) error
	GetExistingResult(ctx context.Context, key string) (uuid.UUID, error)
}
