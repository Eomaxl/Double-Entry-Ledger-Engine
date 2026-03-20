package domain

import (
	"time"

	"github.com/google/uuid"
)

type IdempotencyRecord struct {
	IdempotencyKey string
	TransactionID  uuid.UUID
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

type IdempotencyStatus int

const (
	IdempotencyStatusNew IdempotencyStatus = iota
	IdempotencyStatusExists
)
