package domain

import (
	"time"

	"github.com/google/uuid"
)

type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeEquity    AccountType = "equity"
	AccountTypeRevenue   AccountType = "revenue"
	AccountTypeExpense   AccountType = "expense"
)

func (at AccountType) IsValid() bool {
	switch at {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeRevenue, AccountTypeExpense:
		return true
	default:
		return false
	}
}

type Account struct {
	AccountID   uuid.UUID              `json:"account_id"`
	AccountType AccountType            `json:"account_type"`
	Currencies  []AccountCurrency      `json:"currencies"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type AccountCurrency struct {
	CurrencyCode  string    `json:"currency_code"`
	AllowNegative bool      `json:"allow_negative"`
	CreatedAt     time.Time `json:"created_at"`
}

type CreateAccountRequest struct {
	AccountType AccountType            `json:"account_type"`
	Currencies  []CurrencyConfig       `json:"currencies"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type CurrencyConfig struct {
	CurrencyCode  string `json:"currency_code"`
	AllowNegative bool   `json:"allow_negative"`
}

type UpdateAccountMetadataRequest struct {
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type AccountFilter struct {
	AccountType  *AccountType `json:"account_type,omitempty"`
	CurrencyCode *string      `json:"currency_code,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}
