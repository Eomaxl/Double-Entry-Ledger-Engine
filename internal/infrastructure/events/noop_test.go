package events

import (
	"context"
	"testing"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
)

func TestNoOpEventPublisher_AllMethodsReturnNil(t *testing.T) {
	p := NewNoOpEventPublisher()
	txn := &domain.Transaction{}

	if err := p.PublishTransactionPosted(context.Background(), txn); err != nil {
		t.Fatalf("PublishTransactionPosted error: %v", err)
	}
	if err := p.PublishTransactionSettled(context.Background(), txn); err != nil {
		t.Fatalf("PublishTransactionSettled error: %v", err)
	}
	if err := p.PublishTransactionCancelled(context.Background(), txn); err != nil {
		t.Fatalf("PublishTransactionCancelled error: %v", err)
	}
	if err := p.PublishTransactionReversed(context.Background(), txn); err != nil {
		t.Fatalf("PublishTransactionReversed error: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}
