package transaction

import (
	"testing"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
)

func TestParseUUIDField(t *testing.T) {
	_, err := parseUUIDField("transaction_id", "not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid uuid")
	}
	if _, ok := err.(domain.ValidationError); !ok {
		t.Fatalf("expected domain.ValidationError, got %T", err)
	}

	id, err := parseUUIDField("transaction_id", "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if id.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("unexpected uuid %s", id.String())
	}
}
