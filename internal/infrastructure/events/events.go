package events

import (
	"context"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
)

type EventPublisher interface {
	PublishTransactionPosted(ctx context.Context, txn *domain.Transaction) error
	PublishTransactionSettled(ctx context.Context, txn *domain.Transaction) error
	PublishTransactionCancelled(ctx context.Context, txn *domain.Transaction) error
	PublishTransactionReversed(ctx context.Context, txn *domain.Transaction) error
	Close() error
}
