package middleware

import (
	"runtime/debug"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/errors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ErrorHandler creates a middleware that handles errors and panics
func ErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				handlePanic(c, recovered, logger)
				c.Abort()
			}
		}()

		c.Next()

		if len(c.Errors) > 0 && !c.Writer.Written() {
			handleContextErrors(c, logger)
		}
	}
}

// handlePanic handles panic recovery with standardized error response
func handlePanic(c *gin.Context, recovered interface{}, logger *zap.Logger) {
	// Log the panic with stack trace
	stack := debug.Stack()
	logger.Error("panic recovered",
		zap.Any("panic", recovered),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method),
		zap.String("stack", string(stack)))

	// Create standardized error response
	apiError := errors.InternalError("An unexpected error occurred")
	errors.SendErrorResponse(c, apiError)
}

// handleContextErrors handles errors stored in Gin context
func handleContextErrors(c *gin.Context, logger *zap.Logger) {
	// Get the last error (most recent)
	err := c.Errors.Last().Err

	// Log the error
	logger.Error("request error",
		zap.Error(err),
		zap.String("path", c.Request.URL.Path),
		zap.String("method", c.Request.Method))

	// Convert to standardized error and send response
	apiError := errors.ConvertToAPIError(err)
	errors.SendErrorResponse(c, apiError)
}

// AbortWithError is a helper function to abort request with standardized error
func AbortWithError(c *gin.Context, apiError *errors.APIError) {
	errors.SendErrorResponse(c, apiError)
	c.Abort()
}

// AbortWithErrorCode is a helper function to abort request with error code and message
func AbortWithErrorCode(c *gin.Context, code errors.ErrorCode, message string) {
	apiError := errors.NewAPIError(code, message)
	AbortWithError(c, apiError)
}
