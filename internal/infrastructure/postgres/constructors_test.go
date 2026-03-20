package postgres

import "testing"

func TestConstructors(t *testing.T) {
	if got := NewPostgresAccountRepository(nil, nil); got == nil {
		t.Fatal("expected account repository instance")
	}
	if got := NewPostgresIdempotencyRepository(nil, nil); got == nil {
		t.Fatal("expected idempotency repository instance")
	}
	if got := NewPostgresLedgerRepository(); got == nil {
		t.Fatal("expected ledger repository instance")
	}
}
