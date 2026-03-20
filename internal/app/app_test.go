package app

import (
	"testing"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"go.uber.org/zap"
)

func TestNewEventPublisher_DisabledUsesNoOp(t *testing.T) {
	cfg := &config.Config{
		NATS: config.NATSConfig{
			Enabled: false,
		},
	}

	publisher, err := newEventPublisher(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if publisher == nil {
		t.Fatal("expected non-nil publisher")
	}
}

func TestNewEventPublisher_EnabledReturnsExplicitError(t *testing.T) {
	cfg := &config.Config{
		NATS: config.NATSConfig{
			Enabled: true,
		},
	}

	publisher, err := newEventPublisher(cfg, zap.NewNop())
	if err == nil {
		t.Fatal("expected error when NATS is enabled without concrete implementation")
	}
	if publisher != nil {
		t.Fatal("expected nil publisher when NATS setup fails")
	}
}

func TestShutdownTimeout_ReturnsConfiguredValue(t *testing.T) {
	a := &App{
		shutdownTimeout: 15 * time.Second,
	}

	if got := a.ShutdownTimeout(); got != 15*time.Second {
		t.Fatalf("unexpected shutdown timeout: got=%v want=%v", got, 15*time.Second)
	}
}
