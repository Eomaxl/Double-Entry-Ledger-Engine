package database

import (
	"context"
	"testing"
)

func TestPoolProbe_WithNilPool(t *testing.T) {
	probe := NewPoolProbe(nil)
	if probe == nil {
		t.Fatal("expected non-nil probe")
	}

	if err := probe.Ping(context.Background()); err != nil {
		t.Fatalf("expected nil ping error for nil pool, got %v", err)
	}

	if got := probe.AcquiredConnections(); got != 0 {
		t.Fatalf("expected 0 acquired connections, got %d", got)
	}
}
