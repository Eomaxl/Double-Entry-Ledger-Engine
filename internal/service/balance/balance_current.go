package balance

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// GetCurrentBalance computes the current balance for an account in a specific currency
func (bc *PostgresBalanceCalculator) GetCurrentBalance(ctx context.Context, accountID uuid.UUID, currency string) (*domain.Balance, error) {
	ctx, span := tracing.StartSpan(ctx, "balance.get_current",
		attribute.String("ledger.account_id", accountID.String()),
		attribute.String("ledger.currency", currency),
	)
	defer span.End()

	start := time.Now()
	defer func() {
		bc.metrics.BalanceQueryDuration.Observe(time.Since(start).Seconds())
		bc.metrics.BalanceQueryTotal.WithLabelValues("current", "success").Inc()
	}()

	return bc.getCurrentBalanceWithLocking(ctx, accountID, currency, false, nil)
}

// GetCurrentBalanceWithLock computes the current balance with row-level locking for consistency
func (bc *PostgresBalanceCalculator) GetCurrentBalanceWithLock(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) (*domain.Balance, error) {
	return bc.getCurrentBalanceWithLocking(ctx, accountID, currency, true, tx)
}

// GetCurrentBalanceInTx implements BalanceCalculator for transactional balance checks.
func (bc *PostgresBalanceCalculator) GetCurrentBalanceInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) (*domain.Balance, error) {
	return bc.GetCurrentBalanceWithLock(ctx, tx, accountID, currency)
}

func (bc *PostgresBalanceCalculator) getCurrentBalanceWithLocking(ctx context.Context, accountID uuid.UUID, currency string, useLocking bool, tx pgx.Tx) (*domain.Balance, error) {
	bc.logger.Debug("computing current balance",
		zap.String("account_id", accountID.String()),
		zap.String("currency", currency),
		zap.Bool("use_locking", useLocking))

	var queryRunner interface {
		QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
		Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	}

	if tx != nil {
		queryRunner = tx
	} else {
		queryRunner = bc.pool
	}

	snapshot, err := bc.getLatestSnapshotWithRunner(ctx, queryRunner, accountID, currency)
	if err != nil {
		bc.logger.Debug("no snapshot found, computing from entries",
			zap.String("account_id", accountID.String()),
			zap.String("currency", currency))
	}

	var settledBalance decimal.Decimal
	var entryCount int64
	var startTime *time.Time

	if snapshot != nil {
		settledBalance = snapshot.SettledBalance
		entryCount = snapshot.EntryCount
		startTime = &snapshot.SnapshotAt
		bc.logger.Debug("using snapshot as starting point",
			zap.String("account_id", accountID.String()),
			zap.String("currency", currency),
			zap.Time("snapshot_at", snapshot.SnapshotAt))
	} else {
		settledBalance = decimal.Zero
		entryCount = 0
	}

	settledDelta, settledCount, err := bc.computeSettledBalanceWithRunner(ctx, queryRunner, accountID, currency, startTime, useLocking)
	if err != nil {
		return nil, fmt.Errorf("failed to compute settled balance: %w", err)
	}

	settledBalance = settledBalance.Add(settledDelta)
	entryCount += settledCount

	pendingDebits, pendingCredits, err := bc.computePendingAmountsWithRunner(ctx, queryRunner, accountID, currency, useLocking)
	if err != nil {
		return nil, fmt.Errorf("failed to compute pending amounts: %w", err)
	}

	availableBalance := settledBalance.Add(pendingCredits).Sub(pendingDebits)

	balance := &domain.Balance{
		AccountID:        accountID.String(),
		Currency:         currency,
		SettledBalance:   settledBalance,
		PendingDebits:    pendingDebits,
		PendingCredits:   pendingCredits,
		AvailableBalance: availableBalance,
		EntryCount:       entryCount,
		AsOfTimestamp:    time.Now(),
	}

	bc.logger.Debug("balance computed",
		zap.String("account_id", accountID.String()),
		zap.String("currency", currency),
		zap.String("settled_balance", settledBalance.String()),
		zap.String("available_balance", availableBalance.String()),
		zap.Bool("use_locking", useLocking))

	return balance, nil
}
