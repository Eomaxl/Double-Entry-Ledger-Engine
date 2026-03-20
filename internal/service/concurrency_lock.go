package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// LockAccountForUpdate acquires a row-level lock on an account for balance checks
func (cc *ConcurrencyController) LockAccountForUpdate(ctx context.Context, tx pgx.Tx, accountID string) error {
	lockSQL := `
		SELECT account_id 
		FROM accounts 
		WHERE account_id = $1 
		FOR UPDATE
	`

	var lockedAccountID string
	err := tx.QueryRow(ctx, lockSQL, accountID).Scan(&lockedAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("account not found for locking: %s", accountID)
		}
		cc.logger.Error("failed to acquire account lock",
			zap.Error(err),
			zap.String("account_id", accountID))
		return fmt.Errorf("failed to acquire account lock: %w", err)
	}

	cc.logger.Debug("acquired account lock",
		zap.String("account_id", accountID))

	return nil
}

// LockMultipleAccountsForUpdate acquires row-level locks on multiple accounts in a consistent order
func (cc *ConcurrencyController) LockMultipleAccountsForUpdate(ctx context.Context, tx pgx.Tx, accountIDs []string) error {
	if len(accountIDs) == 0 {
		return nil
	}

	sortedAccountIDs := make([]string, len(accountIDs))
	copy(sortedAccountIDs, accountIDs)
	sort.Strings(sortedAccountIDs)
	uniqueAccountIDs := uniqueStrings(sortedAccountIDs)

	cc.logger.Debug("locking multiple accounts in order",
		zap.Strings("account_ids", uniqueAccountIDs))

	for _, accountID := range uniqueAccountIDs {
		if err := cc.LockAccountForUpdate(ctx, tx, accountID); err != nil {
			return err
		}
	}

	cc.logger.Debug("acquired all account locks",
		zap.Int("account_count", len(uniqueAccountIDs)))

	return nil
}

func uniqueStrings(sortedValues []string) []string {
	if len(sortedValues) == 0 {
		return nil
	}

	unique := make([]string, 0, len(sortedValues))
	last := ""
	for i, value := range sortedValues {
		if i == 0 || value != last {
			unique = append(unique, value)
			last = value
		}
	}

	return unique
}
