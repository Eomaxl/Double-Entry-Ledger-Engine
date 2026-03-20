package balance

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// GetHistoricalBalance computes the balance as of a specific timestamp
func (bc *PostgresBalanceCalculator) GetHistoricalBalance(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (*domain.Balance, error) {
	bc.logger.Debug("computing historical balance",
		zap.String("account_id", accountID.String()),
		zap.String("currency", currency),
		zap.Time("as_of", asOf))

	snapshot, err := bc.getSnapshotBeforeTime(ctx, accountID, currency, asOf)
	if err != nil {
		bc.logger.Debug("no snapshot found for historical query",
			zap.String("account_id", accountID.String()),
			zap.String("currency", currency),
			zap.Time("as_of", asOf))
	}

	var settledBalance decimal.Decimal
	var entryCount int64
	var startTime *time.Time

	if snapshot != nil {
		settledBalance = snapshot.SettledBalance
		entryCount = snapshot.EntryCount
		startTime = &snapshot.SnapshotAt
		bc.logger.Debug("using snapshot for historical query",
			zap.String("account_id", accountID.String()),
			zap.String("currency", currency),
			zap.Time("snapshot_at", snapshot.SnapshotAt))
	} else {
		settledBalance = decimal.Zero
		entryCount = 0
	}

	settledDelta, settledCount, err := bc.computeHistoricalSettledBalance(ctx, accountID, currency, startTime, asOf)
	if err != nil {
		return nil, fmt.Errorf("failed to compute historical settled balance: %w", err)
	}

	settledBalance = settledBalance.Add(settledDelta)
	entryCount += settledCount

	pendingDebits, pendingCredits, err := bc.computeHistoricalPendingAmounts(ctx, accountID, currency, asOf)
	if err != nil {
		return nil, fmt.Errorf("failed to compute historical pending amounts: %w", err)
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
		AsOfTimestamp:    asOf,
	}

	bc.logger.Debug("historical balance computed",
		zap.String("account_id", accountID.String()),
		zap.String("currency", currency),
		zap.Time("as_of", asOf),
		zap.String("settled_balance", settledBalance.String()))

	return balance, nil
}
