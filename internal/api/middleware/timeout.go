package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// Timeout creates a middleware that cancels requests after the specified duration
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			c.AbortWithStatusJSON(504, gin.H{
				"error":   "Request timeout",
				"message": "The request took too long to process",
			})
		}
	}
}
