package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type Balance struct {
	AccountID        string          `json:"account_id"`
	Currency         string          `json:"currency"`
	SettledBalance   decimal.Decimal `json:"settled_balance"`
	PendingDebits    decimal.Decimal `json:"pending_debits"`
	PendingCredits   decimal.Decimal `json:"pending_credits"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	EntryCount       int64           `json:"entry_count"`
	AsOfTimestamp    time.Time       `json:"as_of_timestamp"`
}
