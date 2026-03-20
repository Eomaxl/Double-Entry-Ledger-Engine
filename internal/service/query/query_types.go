package query

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// QueryService defines the interface for transaction query operations
type QueryService interface {
	GetTransaction(ctx context.Context, txnID string) (*domain.Transaction, error)
	ListTransactions(ctx context.Context, filter TransactionFilter) (*TransactionPage, error)
	GetAccountStatement(ctx context.Context, accountID string, filter StatementFilter) (*Statement, error)
}

// TransactionFilter represents filters for transaction queries
type TransactionFilter struct {
	AccountID *string                  `json:"account_id,omitempty"`
	Currency  *string                  `json:"currency,omitempty"`
	State     *domain.TransactionState `json:"state,omitempty"`
	StartDate *time.Time               `json:"start_date,omitempty"`
	EndDate   *time.Time               `json:"end_date,omitempty"`
	Metadata  map[string]string        `json:"metadata,omitempty"`
	Limit     int                      `json:"limit"`
	Offset    int                      `json:"offset"`
	OrderBy   string                   `json:"order_by"`
}

// StatementFilter represents filters for account statement queries
type StatementFilter struct {
	Currency  *string    `json:"currency,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

// TransactionPage represents a paginated list of transactions
type TransactionPage struct {
	Transactions []domain.Transaction `json:"transactions"`
	TotalCount   int64                `json:"total_count"`
	Limit        int                  `json:"limit"`
	Offset       int                  `json:"offset"`
	HasMore      bool                 `json:"has_more"`
}

// Statement represents an account statement with chronological transactions
type Statement struct {
	AccountID    string               `json:"account_id"`
	Currency     *string              `json:"currency,omitempty"`
	Transactions []domain.Transaction `json:"transactions"`
	TotalCount   int64                `json:"total_count"`
	Limit        int                  `json:"limit"`
	Offset       int                  `json:"offset"`
	HasMore      bool                 `json:"has_more"`
}

// PostgresQueryService implements QueryService using PostgreSQL
type PostgresQueryService struct {
	pool   *pgxpool.Pool
	ledger repository.LedgerRepository
	logger *zap.Logger
}

var _ QueryService = (*PostgresQueryService)(nil)

// NewPostgresQueryService creates a new PostgreSQL query service
func NewPostgresQueryService(pool *pgxpool.Pool, ledger repository.LedgerRepository, logger *zap.Logger) *PostgresQueryService {
	return &PostgresQueryService{
		pool:   pool,
		ledger: ledger,
		logger: logger,
	}
}
