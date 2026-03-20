package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetMultiCurrencyBalance computes balances for all currencies of an account in parallel
func (bc *PostgresBalanceCalculator) GetMultiCurrencyBalance(ctx context.Context, accountID uuid.UUID) (map[string]*domain.Balance, error) {
	bc.logger.Debug("computing multi-currency balance",
		zap.String("account_id", accountID.String()))

	currencies, err := bc.getAccountCurrencies(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get account currencies: %w", err)
	}

	if len(currencies) == 0 {
		return make(map[string]*domain.Balance), nil
	}

	type result struct {
		currency string
		balance  *domain.Balance
		err      error
	}

	results := make(chan result, len(currencies))
	var wg sync.WaitGroup

	for _, currency := range currencies {
		wg.Add(1)
		go func(curr string) {
			defer wg.Done()
			balance, err := bc.GetCurrentBalance(ctx, accountID, curr)
			results <- result{currency: curr, balance: balance, err: err}
		}(currency)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	balances := make(map[string]*domain.Balance)
	for res := range results {
		if res.err != nil {
			return nil, fmt.Errorf("failed to compute balance for currency %s: %w", res.currency, res.err)
		}
		balances[res.currency] = res.balance
	}

	bc.logger.Debug("multi-currency balance computed",
		zap.String("account_id", accountID.String()),
		zap.Int("currency_count", len(balances)))

	return balances, nil
}

func (bc *PostgresBalanceCalculator) getAccountCurrencies(ctx context.Context, accountID uuid.UUID) ([]string, error) {
	query := `
		SELECT currency_code
		FROM account_currencies
		WHERE account_id = $1
		ORDER BY currency_code
	`

	rows, err := bc.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to query account currencies: %w", err)
	}
	defer rows.Close()

	currencies := make([]string, 0)
	for rows.Next() {
		var currency string
		if err := rows.Scan(&currency); err != nil {
			return nil, fmt.Errorf("failed to scan currency: %w", err)
		}
		currencies = append(currencies, currency)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating currency rows: %w", err)
	}

	return currencies, nil
}
