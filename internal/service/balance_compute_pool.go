package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func (bc *PostgresBalanceCalculator) computeHistoricalSettledBalance(ctx context.Context, accountID uuid.UUID, currency string, startTime *time.Time, asOf time.Time) (decimal.Decimal, int64, error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN e.entry_type = 'credit' THEN e.amount ELSE 0 END), 0) as total_credits,
			COALESCE(SUM(CASE WHEN e.entry_type = 'debit' THEN e.amount ELSE 0 END), 0) as total_debits,
			COUNT(*) as entry_count
		FROM entries e
		INNER JOIN transactions t ON e.transaction_id = t.transaction_id
		WHERE e.account_id = $1 
			AND e.currency_code = $2 
			AND t.state = 'settled'
			AND t.posted_at <= $3
	`

	args := []interface{}{accountID, currency, asOf}

	if startTime != nil {
		query += " AND t.posted_at > $4"
		args = append(args, *startTime)
	}

	var totalCredits, totalDebits string
	var entryCount int64

	err := bc.pool.QueryRow(ctx, query, args...).Scan(&totalCredits, &totalDebits, &entryCount)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("failed to query historical settled balance: %w", err)
	}

	credits, err := decimal.NewFromString(totalCredits)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("failed to parse total credits: %w", err)
	}

	debits, err := decimal.NewFromString(totalDebits)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("failed to parse total debits: %w", err)
	}

	return credits.Sub(debits), entryCount, nil
}

func (bc *PostgresBalanceCalculator) computeHistoricalPendingAmounts(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (decimal.Decimal, decimal.Decimal, error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN e.entry_type = 'debit' THEN e.amount ELSE 0 END), 0) as pending_debits,
			COALESCE(SUM(CASE WHEN e.entry_type = 'credit' THEN e.amount ELSE 0 END), 0) as pending_credits
		FROM entries e
		INNER JOIN transactions t ON e.transaction_id = t.transaction_id
		WHERE e.account_id = $1 
			AND e.currency_code = $2 
			AND t.state = 'pending'
			AND t.posted_at <= $3
	`

	var pendingDebitsStr, pendingCreditsStr string

	err := bc.pool.QueryRow(ctx, query, accountID, currency, asOf).Scan(&pendingDebitsStr, &pendingCreditsStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("failed to query historical pending amounts: %w", err)
	}

	pendingDebits, err := decimal.NewFromString(pendingDebitsStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("failed to parse pending debits: %w", err)
	}

	pendingCredits, err := decimal.NewFromString(pendingCreditsStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("failed to parse pending credits: %w", err)
	}

	return pendingDebits, pendingCredits, nil
}
