package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
)

func (bc *PostgresBalanceCalculator) computeSettledBalanceWithRunner(ctx context.Context, queryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, accountID uuid.UUID, currency string, startTime *time.Time, useLocking bool) (decimal.Decimal, int64, error) {
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
	`

	if useLocking {
		query += " FOR UPDATE OF e"
	}

	args := []interface{}{accountID, currency}

	if startTime != nil {
		query = query[:len(query)-len(" FOR UPDATE OF e")] + " AND t.posted_at > $3"
		if useLocking {
			query += " FOR UPDATE OF e"
		}
		args = append(args, *startTime)
	}

	var totalCredits, totalDebits string
	var entryCount int64

	err := queryRunner.QueryRow(ctx, query, args...).Scan(&totalCredits, &totalDebits, &entryCount)
	if err != nil {
		return decimal.Zero, 0, fmt.Errorf("failed to query settled balance: %w", err)
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

func (bc *PostgresBalanceCalculator) computePendingAmountsWithRunner(ctx context.Context, queryRunner interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, accountID uuid.UUID, currency string, useLocking bool) (decimal.Decimal, decimal.Decimal, error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN e.entry_type = 'debit' THEN e.amount ELSE 0 END), 0) as pending_debits,
			COALESCE(SUM(CASE WHEN e.entry_type = 'credit' THEN e.amount ELSE 0 END), 0) as pending_credits
		FROM entries e
		INNER JOIN transactions t ON e.transaction_id = t.transaction_id
		WHERE e.account_id = $1 
			AND e.currency_code = $2 
			AND t.state = 'pending'
	`

	if useLocking {
		query += " FOR UPDATE OF e"
	}

	var pendingDebitsStr, pendingCreditsStr string

	err := queryRunner.QueryRow(ctx, query, accountID, currency).Scan(&pendingDebitsStr, &pendingCreditsStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, fmt.Errorf("failed to query pending amounts: %w", err)
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
