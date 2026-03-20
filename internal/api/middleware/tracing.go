package middleware

import (
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Tracing creates a middleware for distributed tracing
func Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start a new span for the HTTP request
		ctx, span := tracing.StartSpan(c.Request.Context(), "http.request",
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.url", c.Request.URL.String()),
			attribute.String("http.route", c.FullPath()),
			attribute.String("http.user_agent", c.Request.UserAgent()),
			attribute.String("http.client_ip", c.ClientIP()),
		)
		defer span.End()

		// Add request ID to span if available
		if requestID := c.GetHeader("X-Request-ID"); requestID != "" {
			span.SetAttributes(attribute.String("http.request_id", requestID))
		}

		// Add correlation ID to span if available
		if correlationID := c.GetHeader("X-Correlation-ID"); correlationID != "" {
			span.SetAttributes(attribute.String("http.correlation_id", correlationID))
		}

		// Update request context with span context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		c.Next()

		// Add response attributes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_size", c.Writer.Size()),
		)

		// Set span status based on HTTP status code
		if c.Writer.Status() >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
			if c.Writer.Status() >= 500 {
				span.SetStatus(codes.Error, "Internal server error")
			} else {
				span.SetStatus(codes.Error, "Client error")
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		// Add any errors from the request context
		if len(c.Errors) > 0 {
			span.SetAttributes(attribute.Bool("error", true))
			for _, err := range c.Errors {
				span.RecordError(err.Err)
			}
		}
	}
}
