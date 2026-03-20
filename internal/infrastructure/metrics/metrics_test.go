package metrics

import "testing"

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("expected non-nil metrics")
	}
	if m.TransactionTotal == nil || m.BalanceQueryTotal == nil || m.HTTPRequestTotal == nil {
		t.Fatal("expected key metric collectors to be initialized")
	}
}
