package events

import (
	"context"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
)

type NoOpEventPublisher struct{}

func NewNoOpEventPublisher() *NoOpEventPublisher {
	return &NoOpEventPublisher{}
}

func (n *NoOpEventPublisher) PublishTransactionPosted(ctx context.Context, txn *domain.Transaction) error {
	return nil
}

func (n *NoOpEventPublisher) PublishTransactionSettled(ctx context.Context, txn *domain.Transaction) error {
	return nil
}

func (n *NoOpEventPublisher) PublishTransactionCancelled(ctx context.Context, txn *domain.Transaction) error {
	return nil
}

func (n *NoOpEventPublisher) PublishTransactionReversed(ctx context.Context, txn *domain.Transaction) error {
	return nil
}

func (n *NoOpEventPublisher) Close() error {
	return nil
}

var _ EventPublisher = (*NoOpEventPublisher)(nil)
