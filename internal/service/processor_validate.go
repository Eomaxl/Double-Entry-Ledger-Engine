package service

import (
	"context"
	"fmt"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type accountCurrencyKey struct {
	accountID string
	currency  string
}

func (p *PostgresTransactionProcessor) validateAccountsAndBalancesWithLocking(ctx context.Context, tx pgx.Tx, req PostTransactionRequest) error {
	return p.validateAccountsForPost(ctx, tx, req, true)
}

func (p *PostgresTransactionProcessor) validateAccountsAndBalances(ctx context.Context, tx pgx.Tx, req PostTransactionRequest) error {
	return p.validateAccountsForPost(ctx, tx, req, false)
}

func (p *PostgresTransactionProcessor) validateAccountsForPost(ctx context.Context, tx pgx.Tx, req PostTransactionRequest, withLocks bool) error {
	balanceImpacts := make(map[accountCurrencyKey]decimal.Decimal)
	accountIDs := make(map[string]bool)

	for _, entry := range req.Entries {
		accountIDs[entry.AccountID] = true
		key := accountCurrencyKey{accountID: entry.AccountID, currency: entry.CurrencyCode}
		impact := balanceImpacts[key]
		if entry.EntryType == domain.EntryTypeCredit {
			impact = impact.Add(entry.Amount)
		} else {
			impact = impact.Sub(entry.Amount)
		}
		balanceImpacts[key] = impact
	}

	if withLocks {
		slice := make([]string, 0, len(accountIDs))
		for id := range accountIDs {
			slice = append(slice, id)
		}
		if err := p.concurrencyController.LockMultipleAccountsForUpdate(ctx, tx, slice); err != nil {
			return err
		}
	}

	for accountIDStr := range accountIDs {
		accountID, err := uuid.Parse(accountIDStr)
		if err != nil {
			return domain.ValidationError{
				Field:   "account_id",
				Message: fmt.Sprintf("invalid account ID format: %s", accountIDStr),
			}
		}

		account, err := p.accountRepo.GetAccountInTx(ctx, tx, accountID)
		if err != nil {
			return domain.ValidationError{
				Field:   "account_id",
				Message: fmt.Sprintf("account not found: %s", accountIDStr),
			}
		}

		currencyPolicies := make(map[string]bool)
		for _, curr := range account.Currencies {
			currencyPolicies[curr.CurrencyCode] = curr.AllowNegative
		}

		for key, impact := range balanceImpacts {
			if key.accountID != accountIDStr {
				continue
			}

			allowNegative, exists := currencyPolicies[key.currency]
			if !exists {
				return domain.ValidationError{
					Field:   "currency_code",
					Message: fmt.Sprintf("account %s does not support currency %s", accountIDStr, key.currency),
				}
			}

			if !allowNegative && impact.IsNegative() {
				balance, err := p.balanceCalculator.GetCurrentBalanceInTx(ctx, tx, accountID, key.currency)
				if err != nil {
					p.logger.Error("failed to get current balance",
						zap.Error(err),
						zap.String("account_id", accountIDStr),
						zap.String("currency", key.currency))
					return fmt.Errorf("failed to get current balance: %w", err)
				}

				newBalance := balance.AvailableBalance.Add(impact)
				if newBalance.IsNegative() {
					return &InsufficientBalanceError{
						AccountID:        accountIDStr,
						Currency:         key.currency,
						RequiredAmount:   impact.Abs(),
						AvailableBalance: balance.AvailableBalance,
					}
				}
			}
		}
	}

	return nil
}
