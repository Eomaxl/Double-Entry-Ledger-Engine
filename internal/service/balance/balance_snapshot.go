package balance

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func (bc *PostgresBalanceCalculator) getSnapshotBeforeTime(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (*balanceSnapshot, error) {
	query := `
		SELECT settled_balance, pending_debits, pending_credits, entry_count, snapshot_at
		FROM balance_snapshots
		WHERE account_id = $1 AND currency_code = $2 AND snapshot_at <= $3
		ORDER BY snapshot_at DESC
		LIMIT 1
	`

	var snapshot balanceSnapshot
	var settledBalance, pendingDebits, pendingCredits string

	err := bc.pool.QueryRow(ctx, query, accountID, currency, asOf).Scan(
		&settledBalance,
		&pendingDebits,
		&pendingCredits,
		&snapshot.EntryCount,
		&snapshot.SnapshotAt,
	)

	if err != nil {
		return nil, err
	}

	snapshot.SettledBalance, err = decimal.NewFromString(settledBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse settled balance: %w", err)
	}

	snapshot.PendingDebits, err = decimal.NewFromString(pendingDebits)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pending debits: %w", err)
	}

	snapshot.PendingCredits, err = decimal.NewFromString(pendingCredits)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pending credits: %w", err)
	}

	return &snapshot, nil
}

func (bc *PostgresBalanceCalculator) getLatestSnapshotWithRunner(ctx context.Context, queryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, accountID uuid.UUID, currency string) (*balanceSnapshot, error) {
	query := `
		SELECT settled_balance, pending_debits, pending_credits, entry_count, snapshot_at
		FROM balance_snapshots
		WHERE account_id = $1 AND currency_code = $2
		ORDER BY snapshot_at DESC
		LIMIT 1
	`

	var snapshot balanceSnapshot
	var settledBalance, pendingDebits, pendingCredits string

	err := queryRunner.QueryRow(ctx, query, accountID, currency).Scan(
		&settledBalance,
		&pendingDebits,
		&pendingCredits,
		&snapshot.EntryCount,
		&snapshot.SnapshotAt,
	)

	if err != nil {
		return nil, err
	}

	snapshot.SettledBalance, err = decimal.NewFromString(settledBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse settled balance: %w", err)
	}

	snapshot.PendingDebits, err = decimal.NewFromString(pendingDebits)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pending debits: %w", err)
	}

	snapshot.PendingCredits, err = decimal.NewFromString(pendingCredits)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pending credits: %w", err)
	}

	return &snapshot, nil
}
