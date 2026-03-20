package database

import (
	"context"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
)

type InstrumentedQuerier struct {
	querier pgx.Tx
	metrics *metrics.Metrics
}

func NewInstrumentedQueries(querier pgx.Tx, metrics *metrics.Metrics) *InstrumentedQuerier {
	return &InstrumentedQuerier{
		querier: querier,
		metrics: metrics,
	}
}

func (iq *InstrumentedQuerier) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	ctx, span := tracing.StartDatabaseSpan(ctx, "query", extractTableFromSQL(sql), attribute.String("db.statement", sql), attribute.Int("db.args_count", len(args)))
	defer span.End()

	// Record query metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if iq.metrics != nil {
			iq.metrics.DBQueryDuration.WithLabelValues("query").Observe(duration.Seconds())
		}
	}()

	// Execute Query
	rows, err := iq.querier.Query(ctx, sql, args...)

	// Add error to span if present
	if err != nil {
		tracing.AddErrorAttributes(span, err)
	}

	return rows, err
}

func (iq *InstrumentedQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	// Start tracing span
	ctx, span := tracing.StartDatabaseSpan(ctx, "query_row", extractTableFromSQL(sql),
		attribute.String("db.statement", sql),
		attribute.Int("db.args_count", len(args)),
	)
	defer span.End()

	// Record query metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if iq.metrics != nil {
			iq.metrics.DBQueryDuration.WithLabelValues("query_row").Observe(duration.Seconds())
		}
	}()

	// Execute query
	row := iq.querier.QueryRow(ctx, sql, args...)

	return row
}

// Exec executes a command with observability
func (iq *InstrumentedQuerier) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	// Start tracing span
	ctx, span := tracing.StartDatabaseSpan(ctx, "exec", extractTableFromSQL(sql),
		attribute.String("db.statement", sql),
		attribute.Int("db.args_count", len(args)),
	)
	defer span.End()

	// Record query metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if iq.metrics != nil {
			iq.metrics.DBQueryDuration.WithLabelValues("exec").Observe(duration.Seconds())
		}
	}()

	// Execute command
	result, err := iq.querier.Exec(ctx, sql, args...)

	// Add error to span if present
	if err != nil {
		tracing.AddErrorAttributes(span, err)
	}

	return result, err
}

func extractTableFromSQL(sql string) string {
	// Simple heuristic - look for common patterns
	// This is a basic implementation; a more sophisticated parser could be used

	// Look for INSERT INTO, UPDATE, DELETE FROM, SELECT FROM patterns
	patterns := []string{
		"INSERT INTO ",
		"UPDATE ",
		"DELETE FROM ",
		"FROM ",
	}

	for _, pattern := range patterns {
		if idx := findCaseInsensitive(sql, pattern); idx != -1 {
			start := idx + len(pattern)
			end := start

			// Find the end of the table name (space, comma, or parenthesis)
			for end < len(sql) && sql[end] != ' ' && sql[end] != ',' && sql[end] != '(' && sql[end] != '\n' && sql[end] != '\t' {
				end++
			}

			if end > start {
				return sql[start:end]
			}
		}
	}

	return "unknown"
}

// findCaseInsensitive finds a substring in a case-insensitive manner
func findCaseInsensitive(s, substr string) int {
	sLower := make([]byte, len(s))
	substrLower := make([]byte, len(substr))

	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			sLower[i] = s[i] + 32
		} else {
			sLower[i] = s[i]
		}
	}

	for i := 0; i < len(substr); i++ {
		if substr[i] >= 'A' && substr[i] <= 'Z' {
			substrLower[i] = substr[i] + 32
		} else {
			substrLower[i] = substr[i]
		}
	}

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		match := true
		for j := 0; j < len(substrLower); j++ {
			if sLower[i+j] != substrLower[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}

	return -1
}
