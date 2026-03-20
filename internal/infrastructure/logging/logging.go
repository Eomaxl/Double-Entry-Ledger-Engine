package logging

import (
	"context"
	"strings"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"go.uber.org/zap"
)

func NewLogger(cfg config.LoggingConfig) (*zap.Logger, error) {
	switch strings.ToLower(cfg.Format) {
	case "console":
		loggerCfg := zap.NewDevelopmentConfig()
		loggerCfg.Level = parseLevel(cfg.Level)
		return loggerCfg.Build()
	default:
		loggerCfg := zap.NewProductionConfig()
		loggerCfg.Level = parseLevel(cfg.Level)
		return loggerCfg.Build()
	}
}

func LogTransactionSubmission(ctx context.Context, logger *zap.Logger, transactionID, idempotencyKey string, entryCount int) {
	fields := []zap.Field{
		zap.String("event", "transaction_submission"),
		zap.Int("entry_count", entryCount),
	}
	if transactionID != "" {
		fields = append(fields, zap.String("transaction_id", transactionID))
	}
	if idempotencyKey != "" {
		fields = append(fields, zap.String("idempotency_key", idempotencyKey))
	}
	appendRequestContext(ctx, &fields)
	logger.Info("transaction submitted", fields...)
}

func LogValidationFailure(ctx context.Context, logger *zap.Logger, operation string, details map[string]interface{}, err error) {
	fields := []zap.Field{
		zap.String("event", "validation_failure"),
		zap.String("operation", operation),
		zap.Error(err),
	}
	appendDetails(&fields, details)
	appendRequestContext(ctx, &fields)
	logger.Warn("validation failed", fields...)
}

func LogError(ctx context.Context, logger *zap.Logger, operation string, err error, details map[string]interface{}) {
	fields := []zap.Field{
		zap.String("event", "operation_error"),
		zap.String("operation", operation),
		zap.Error(err),
	}
	appendDetails(&fields, details)
	appendRequestContext(ctx, &fields)
	logger.Error("operation failed", fields...)
}

func appendRequestContext(ctx context.Context, fields *[]zap.Field) {
	if requestID, ok := ctx.Value("request_id").(string); ok && requestID != "" {
		*fields = append(*fields, zap.String("request_id", requestID))
	}
	if correlationID, ok := ctx.Value("correlation_id").(string); ok && correlationID != "" {
		*fields = append(*fields, zap.String("correlation_id", correlationID))
	}
}

func appendDetails(fields *[]zap.Field, details map[string]interface{}) {
	for key, value := range details {
		*fields = append(*fields, zap.Any(key, value))
	}
}

func parseLevel(level string) zap.AtomicLevel {
	switch strings.ToLower(level) {
	case "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		return zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}
