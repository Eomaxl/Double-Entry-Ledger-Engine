package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

func TestExecuteWithRetry_SucceedsAfterRetryableError(t *testing.T) {
	cc := NewConcurrencyController(zap.NewNop())
	cfg := RetryConfig{
		MaxRetries:      2,
		BaseDelay:       time.Microsecond,
		MaxDelay:        time.Millisecond,
		JitterFactor:    0,
		RetryableErrors: []string{"40001"},
	}

	attempts := 0
	err := cc.ExecuteWithRetry(context.Background(), cfg, func(ctx context.Context) error {
		attempts++
		if attempts == 1 {
			return &pgconn.PgError{Code: "40001"}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestExecuteWithRetry_NonRetryableFailsImmediately(t *testing.T) {
	cc := NewConcurrencyController(zap.NewNop())
	cfg := DefaultRetryConfig()

	attempts := 0
	sentinel := errors.New("boom")
	err := cc.ExecuteWithRetry(context.Background(), cfg, func(ctx context.Context) error {
		attempts++
		return sentinel
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
}

func TestExecuteWithRetry_ContextCancelledDuringBackoff(t *testing.T) {
	cc := NewConcurrencyController(zap.NewNop())
	cfg := RetryConfig{
		MaxRetries:      5,
		BaseDelay:       50 * time.Millisecond,
		MaxDelay:        50 * time.Millisecond,
		JitterFactor:    0,
		RetryableErrors: []string{"40001"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := true
	err := cc.ExecuteWithRetry(ctx, cfg, func(ctx context.Context) error {
		if first {
			first = false
			cancel()
			return &pgconn.PgError{Code: "40001"}
		}
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
