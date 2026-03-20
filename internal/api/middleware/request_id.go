package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID creates a middleware that adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already provided
		requestID := c.GetHeader("X-Request-ID")

		// Generate a new one if not provided
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set the request ID in the response header
		c.Header("X-Request-ID", requestID)

		// Store in context for use by handlers and other middleware
		c.Set("request_id", requestID)

		c.Next()
	}
}
