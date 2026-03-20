package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger creates a middleware for logging HTTP requests with correlation IDs
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		// Get correlation ID from context
		correlationID := ""
		if requestID := param.Request.Header.Get("X-Request-ID"); requestID != "" {
			correlationID = requestID
		}
		if cid, exists := param.Keys["correlation_id"]; exists {
			if cidStr, ok := cid.(string); ok {
				correlationID = cidStr
			}
		}

		// Build structured log fields
		fields := []zap.Field{
			zap.String("event", "http_request"),
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
			zap.String("user_agent", param.Request.UserAgent()),
			zap.Time("timestamp", param.TimeStamp),
		}

		// Add correlation ID if available
		if correlationID != "" {
			fields = append(fields, zap.String("correlation_id", correlationID))
		}

		// Add request ID if available
		if requestID := param.Request.Header.Get("X-Request-ID"); requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}

		// Add error message if present
		if param.ErrorMessage != "" {
			fields = append(fields, zap.String("error", param.ErrorMessage))
		}

		// Add request size if available
		if param.Request.ContentLength > 0 {
			fields = append(fields, zap.Int64("request_size", param.Request.ContentLength))
		}

		// Add response size if available
		if param.BodySize > 0 {
			fields = append(fields, zap.Int("response_size", param.BodySize))
		}

		// Log at appropriate level based on status code
		if param.StatusCode >= 500 {
			logger.Error("HTTP request completed", fields...)
		} else if param.StatusCode >= 400 {
			logger.Warn("HTTP request completed", fields...)
		} else {
			logger.Info("HTTP request completed", fields...)
		}

		return "" // Return empty string since we're using structured logging
	})
}

// CorrelationID creates a middleware that ensures correlation ID is available in context
func CorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get correlation ID from request header or use request ID
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = c.GetHeader("X-Request-ID")
		}
		if correlationID == "" {
			// Use request ID from context if available
			if requestID, exists := c.Get("request_id"); exists {
				if requestIDStr, ok := requestID.(string); ok {
					correlationID = requestIDStr
				}
			}
		}

		// Store correlation ID in context for use by handlers
		if correlationID != "" {
			c.Set("correlation_id", correlationID)

			// Also add to request context for downstream services
			ctx := context.WithValue(c.Request.Context(), "correlation_id", correlationID)
			c.Request = c.Request.WithContext(ctx)
		}

		c.Next()
	}
}
