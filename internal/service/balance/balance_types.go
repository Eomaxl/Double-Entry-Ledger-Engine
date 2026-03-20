package balance

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// BalanceCalculator defines the interface for balance computation operations
type BalanceCalculator interface {
	GetCurrentBalance(ctx context.Context, accountID uuid.UUID, currency string) (*domain.Balance, error)
	GetCurrentBalanceInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) (*domain.Balance, error)
	GetHistoricalBalance(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (*domain.Balance, error)
	GetMultiCurrencyBalance(ctx context.Context, accountID uuid.UUID) (map[string]*domain.Balance, error)
}

// PostgresBalanceCalculator implements BalanceCalculator using PostgreSQL
type PostgresBalanceCalculator struct {
	pool    *pgxpool.Pool
	logger  *zap.Logger
	metrics *metrics.Metrics
}

var _ BalanceCalculator = (*PostgresBalanceCalculator)(nil)

type balanceSnapshot struct {
	SettledBalance decimal.Decimal
	PendingDebits  decimal.Decimal
	PendingCredits decimal.Decimal
	EntryCount     int64
	SnapshotAt     time.Time
}

// NewPostgresBalanceCalculator creates a new PostgreSQL balance calculator
func NewPostgresBalanceCalculator(pool *pgxpool.Pool, logger *zap.Logger, metrics *metrics.Metrics) *PostgresBalanceCalculator {
	return &PostgresBalanceCalculator{
		pool:    pool,
		logger:  logger,
		metrics: metrics,
	}
}
