package middleware

import (
	"strconv"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/gin-gonic/gin"
)

// Metrics creates a middleware for collecting comprehensive HTTP metrics
func Metrics(metricSet *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start)
		status := strconv.Itoa(c.Writer.Status())

		// Normalize endpoint for metrics (replace path parameters with placeholders)
		endpoint := normalizeEndpoint(c.FullPath())

		// Record HTTP request metrics
		metricSet.HTTPRequestTotal.WithLabelValues(c.Request.Method, endpoint, status).Inc()
		metricSet.HTTPRequestDuration.WithLabelValues(c.Request.Method, endpoint).Observe(duration.Seconds())

		// Record transaction-specific metrics based on endpoint
		switch endpoint {
		case "/v1/transactions":
			if c.Request.Method == "POST" {
				if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
					metricSet.TransactionTotal.WithLabelValues("settled", "success").Inc()
				} else {
					metricSet.TransactionTotal.WithLabelValues("unknown", "error").Inc()
				}
				metricSet.TransactionDuration.WithLabelValues("post_transaction").Observe(duration.Seconds())
			}
		case "/v1/transactions/batch":
			if c.Request.Method == "POST" {
				if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
					metricSet.BatchTransactionTotal.Inc()
				}
				metricSet.TransactionDuration.WithLabelValues("post_batch").Observe(duration.Seconds())
			}
		case "/v1/transactions/:id/settle":
			if c.Request.Method == "POST" {
				metricSet.TransactionDuration.WithLabelValues("settle_transaction").Observe(duration.Seconds())
			}
		case "/v1/transactions/:id/cancel":
			if c.Request.Method == "POST" {
				metricSet.TransactionDuration.WithLabelValues("cancel_transaction").Observe(duration.Seconds())
			}
		case "/v1/transactions/:id/reverse":
			if c.Request.Method == "POST" {
				metricSet.TransactionDuration.WithLabelValues("reverse_transaction").Observe(duration.Seconds())
			}
		case "/v1/accounts/:id/balances", "/v1/accounts/:id/balances/:currency", "/v1/accounts/:id/balances/:currency/history":
			if c.Request.Method == "GET" {
				queryType := "current"
				if endpoint == "/v1/accounts/:id/balances/:currency/history" {
					queryType = "historical"
				} else if endpoint == "/v1/accounts/:id/balances/:currency" {
					queryType = "single_currency"
				} else {
					queryType = "multi_currency"
				}

				status := "success"
				if c.Writer.Status() >= 400 {
					status = "error"
				}

				metricSet.BalanceQueryTotal.WithLabelValues(queryType, status).Inc()
				metricSet.BalanceQueryDuration.Observe(duration.Seconds())
			}
		}
	}
}

// normalizeEndpoint converts Gin route patterns to normalized endpoint names for metrics
func normalizeEndpoint(fullPath string) string {
	if fullPath == "" {
		return "unknown"
	}

	// Gin already provides the route pattern with :param syntax
	// We can use this directly for metrics
	return fullPath
}
