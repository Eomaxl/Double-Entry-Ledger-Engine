package logging

import (
	"context"
	"testing"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"go.uber.org/zap"
)

func TestNewLogger_SupportedFormats(t *testing.T) {
	logger, err := NewLogger(config.LoggingConfig{Level: "debug", Format: "console"})
	if err != nil {
		t.Fatalf("console logger error: %v", err)
	}
	_ = logger.Sync()

	logger, err = NewLogger(config.LoggingConfig{Level: "info", Format: "json"})
	if err != nil {
		t.Fatalf("json logger error: %v", err)
	}
	_ = logger.Sync()
}

func TestParseLevel_DefaultsToInfo(t *testing.T) {
	if got := parseLevel("unknown").Level(); got != zap.InfoLevel {
		t.Fatalf("expected info level, got %v", got)
	}
}

func TestLoggingHelpers_DoNotPanic(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.WithValue(context.Background(), "request_id", "r-1")
	ctx = context.WithValue(ctx, "correlation_id", "c-1")

	LogTransactionSubmission(ctx, logger, "txn-1", "idem-1", 2)
	LogValidationFailure(ctx, logger, "post_txn", map[string]interface{}{"field": "state"}, context.Canceled)
	LogError(ctx, logger, "query_balance", context.DeadlineExceeded, map[string]interface{}{"account_id": "a-1"})
}
