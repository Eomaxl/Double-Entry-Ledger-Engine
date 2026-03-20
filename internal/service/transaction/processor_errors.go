package transaction

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// InsufficientBalanceError represents an error when an account has insufficient balance
type InsufficientBalanceError struct {
	AccountID        string
	Currency         string
	RequiredAmount   decimal.Decimal
	AvailableBalance decimal.Decimal
}

func (e *InsufficientBalanceError) Error() string {
	return fmt.Sprintf("insufficient balance: account %s has %s %s available, but %s is required",
		e.AccountID, e.AvailableBalance.String(), e.Currency, e.RequiredAmount.String())
}
