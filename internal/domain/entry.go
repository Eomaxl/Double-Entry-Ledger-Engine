package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

type EntryType string

const (
	EntryTypeDebit  EntryType = "debit"
	EntryTypeCredit EntryType = "credit"
)

func (e EntryType) IsValid() bool {
	switch e {
	case EntryTypeCredit, EntryTypeDebit:
		return true
	default:
		return false
	}
}

type Entry struct {
	EntryID       string          `json:"entry_id"`
	TransactionID string          `json:"transaction_id"`
	AccountID     string          `json:"account_id"`
	CurrencyCode  string          `json:"currency_code"`
	Amount        decimal.Decimal `json:"amount"`
	EntryType     EntryType       `json:"entry_type"`
	Description   string          `json:"description,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}
