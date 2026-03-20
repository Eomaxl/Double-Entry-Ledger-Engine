package transaction

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/events"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// TransactionProcessor defines the interface for transaction operations
type TransactionProcessor interface {
	PostTransaction(ctx context.Context, req PostTransactionRequest) (*domain.Transaction, error)
	PostBatch(ctx context.Context, reqs []PostTransactionRequest) ([]domain.Transaction, error)
	SettlePendingTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	CancelPendingTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error)
	ReverseTransaction(ctx context.Context, originalTransactionID string, req ReversalRequest) (*domain.Transaction, error)
}

type BalanceCalculator interface {
	GetCurrentBalance(ctx context.Context, accountID uuid.UUID, currency string) (*domain.Balance, error)
	GetCurrentBalanceInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) (*domain.Balance, error)
	GetHistoricalBalance(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (*domain.Balance, error)
	GetMultiCurrencyBalance(ctx context.Context, accountID uuid.UUID) (map[string]*domain.Balance, error)
}

// PostgresTransactionProcessor implements TransactionProcessor using PostgreSQL
type PostgresTransactionProcessor struct {
	pool                  *pgxpool.Pool
	validator             *domain.TransactionValidator
	accountRepo           repository.AccountRepository
	idempotencyRepo       repository.IdempotencyRepository
	balanceCalculator     BalanceCalculator
	ledgerRepo            repository.LedgerRepository
	eventPublisher        events.EventPublisher
	concurrencyController *ConcurrencyController
	idempotencyTTL        time.Duration
	logger                *zap.Logger
	metrics               *metrics.Metrics
}

var _ TransactionProcessor = (*PostgresTransactionProcessor)(nil)

// PostTransactionRequest represents a request to post a new transaction
type PostTransactionRequest struct {
	IdempotencyKey *string                 `json:"idempotency_key,omitempty"`
	State          domain.TransactionState `json:"state"`
	Entries        []EntryRequest          `json:"entries"`
	Metadata       map[string]interface{}  `json:"metadata,omitempty"`
}

// EntryRequest represents an entry in a transaction request
type EntryRequest struct {
	AccountID    string           `json:"account_id"`
	CurrencyCode string           `json:"currency_code"`
	Amount       decimal.Decimal  `json:"amount"`
	EntryType    domain.EntryType `json:"entry_type"`
	Description  string           `json:"description,omitempty"`
}

// ReversalRequest represents a request to reverse a transaction
type ReversalRequest struct {
	IdempotencyKey *string                 `json:"idempotency_key,omitempty"`
	State          domain.TransactionState `json:"state"`
	Description    string                  `json:"description,omitempty"`
	Metadata       map[string]interface{}  `json:"metadata,omitempty"`
}

// NewPostgresTransactionProcessor creates a new PostgreSQL transaction processor
func NewPostgresTransactionProcessor(
	pool *pgxpool.Pool,
	validator *domain.TransactionValidator,
	accountRepo repository.AccountRepository,
	idempotencyRepo repository.IdempotencyRepository,
	balanceCalculator BalanceCalculator,
	ledgerRepo repository.LedgerRepository,
	eventPublisher events.EventPublisher,
	idempotencyTTL time.Duration,
	logger *zap.Logger,
	metrics *metrics.Metrics,
) *PostgresTransactionProcessor {
	return &PostgresTransactionProcessor{
		pool:                  pool,
		validator:             validator,
		accountRepo:           accountRepo,
		idempotencyRepo:       idempotencyRepo,
		balanceCalculator:     balanceCalculator,
		ledgerRepo:            ledgerRepo,
		eventPublisher:        eventPublisher,
		concurrencyController: NewConcurrencyController(logger),
		idempotencyTTL:        idempotencyTTL,
		logger:                logger,
		metrics:               metrics,
	}
}
