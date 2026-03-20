package tracing

import (
	"context"
	"testing"
)

func TestInitTracing_Disabled(t *testing.T) {
	shutdown, err := InitTracing(TracingConfig{Enabled: false})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected non-nil shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("expected nil shutdown error, got %v", err)
	}
}

func TestHeaderCarrier_Basics(t *testing.T) {
	h := &HeaderCarrier{headers: map[string]string{}}
	h.Set("k1", "v1")
	if got := h.Get("k1"); got != "v1" {
		t.Fatalf("expected v1, got %s", got)
	}
	if len(h.Keys()) != 1 {
		t.Fatalf("expected 1 key, got %d", len(h.Keys()))
	}
}
