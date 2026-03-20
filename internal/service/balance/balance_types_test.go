package balance

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewPostgresBalanceCalculator(t *testing.T) {
	bc := NewPostgresBalanceCalculator(nil, zap.NewNop(), nil)
	if bc == nil {
		t.Fatal("expected non-nil balance calculator")
	}
}
