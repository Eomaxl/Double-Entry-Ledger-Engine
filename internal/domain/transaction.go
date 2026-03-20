package domain

import "time"

type TransactionState string

const (
	TransactionStatePending   TransactionState = "pending"
	TransactionStateSettled   TransactionState = "settled"
	TransactionStateCancelled TransactionState = "cancelled"
)

func (s TransactionState) IsValid() bool {
	switch s {
	case TransactionStatePending, TransactionStateSettled, TransactionStateCancelled:
		return true
	default:
		return false
	}
}

type Transaction struct {
	TransactionID           string                 `json:"transaction_id"`
	IdempotencyKey          *string                `json:"idempotency_key,omitempty"`
	State                   TransactionState       `json:"state"`
	PostedAt                time.Time              `json:"posted_at"`
	SettledAt               *time.Time             `json:"settled_at,omitempty"`
	ReversedByTransactionID *string                `json:"reversed_by_transaction_id,omitempty"`
	ReversesTransactionID   *string                `json:"reverses_transaction_id,omitempty"`
	Entries                 []Entry                `json:"entries"`
	Metadata                map[string]interface{} `json:"metadata,omitempty"`
}
