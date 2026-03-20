package transaction

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// ConcurrencyController provides concurrency control mechanisms for database operations
type ConcurrencyController struct {
	logger *zap.Logger
}

// NewConcurrencyController creates a new concurrency controller
func NewConcurrencyController(logger *zap.Logger) *ConcurrencyController {
	return &ConcurrencyController{
		logger: logger,
	}
}

// RetryConfig defines configuration for retry logic
type RetryConfig struct {
	MaxRetries      int
	BaseDelay       time.Duration
	MaxDelay        time.Duration
	JitterFactor    float64
	RetryableErrors []string
}

// DefaultRetryConfig returns a default retry configuration for serialization failures
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   5,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		JitterFactor: 0.1,
		RetryableErrors: []string{
			"40001",
			"40P01",
		},
	}
}

// RetryableOperation represents an operation that can be retried on serialization failures
type RetryableOperation func(ctx context.Context) error

// ExecuteWithRetry executes an operation with retry logic for serialization failures
func (cc *ConcurrencyController) ExecuteWithRetry(ctx context.Context, config RetryConfig, operation RetryableOperation) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := cc.calculateBackoffDelay(config, attempt)

			cc.logger.Debug("retrying operation after serialization failure",
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
				zap.Error(lastErr))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := operation(ctx)
		if err == nil {
			if attempt > 0 {
				cc.logger.Info("operation succeeded after retry",
					zap.Int("attempts", attempt+1))
			}
			return nil
		}

		lastErr = err

		if !cc.isRetryableError(err, config.RetryableErrors) {
			cc.logger.Debug("error is not retryable, failing immediately",
				zap.Error(err))
			return err
		}

		cc.logger.Debug("retryable error encountered",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_retries", config.MaxRetries))
	}

	cc.logger.Warn("operation failed after all retry attempts",
		zap.Int("max_retries", config.MaxRetries),
		zap.Error(lastErr))

	return fmt.Errorf("operation failed after %d retries: %w", config.MaxRetries, lastErr)
}

func (cc *ConcurrencyController) calculateBackoffDelay(config RetryConfig, attempt int) time.Duration {
	delay := time.Duration(float64(config.BaseDelay) * math.Pow(2, float64(attempt-1)))

	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	if config.JitterFactor > 0 {
		jitter := time.Duration(float64(delay) * config.JitterFactor * rand.Float64())
		delay += jitter
	}

	return delay
}

func (cc *ConcurrencyController) isRetryableError(err error, retryableErrors []string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	for _, code := range retryableErrors {
		if pgErr.Code == code {
			return true
		}
	}

	return false
}
