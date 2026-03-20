package errors

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service"
	"github.com/gin-gonic/gin"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Validation error codes (400 Bad Request)
	ErrorCodeInvalidRequest        ErrorCode = "INVALID_REQUEST"
	ErrorCodeMissingField          ErrorCode = "MISSING_FIELD"
	ErrorCodeInvalidCurrency       ErrorCode = "INVALID_CURRENCY"
	ErrorCodeNegativeAmount        ErrorCode = "NEGATIVE_AMOUNT"
	ErrorCodePrecisionExceeded     ErrorCode = "PRECISION_EXCEEDED"
	ErrorCodeUnbalancedTransaction ErrorCode = "UNBALANCED_TRANSACTION"
	ErrorCodeInvalidState          ErrorCode = "INVALID_STATE"
	ErrorCodeInvalidFormat         ErrorCode = "INVALID_FORMAT"

	// Business rule violation codes (422 Unprocessable Entity)
	ErrorCodeInsufficientBalance   ErrorCode = "INSUFFICIENT_BALANCE"
	ErrorCodeNegativeBalancePolicy ErrorCode = "NEGATIVE_BALANCE_POLICY"
	ErrorCodeAccountNotFound       ErrorCode = "ACCOUNT_NOT_FOUND"
	ErrorCodeTransactionNotFound   ErrorCode = "TRANSACTION_NOT_FOUND"
	ErrorCodeInvalidReversal       ErrorCode = "INVALID_REVERSAL"
	ErrorCodeUnsupportedCurrency   ErrorCode = "UNSUPPORTED_CURRENCY"

	// Idempotency conflict codes (409 Conflict)
	ErrorCodeIdempotencyConflict ErrorCode = "IDEMPOTENCY_CONFLICT"
	ErrorCodeDuplicateKey        ErrorCode = "DUPLICATE_KEY"

	// Authentication/Authorization codes (401/403)
	ErrorCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrorCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrorCodeInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"

	// System error codes (500 Internal Server Error)
	ErrorCodeDatabaseError      ErrorCode = "DATABASE_ERROR"
	ErrorCodeEventEmissionError ErrorCode = "EVENT_EMISSION_ERROR"
	ErrorCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

// APIError represents a standardized API error
type APIError struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorResponse represents the complete error response structure
type ErrorResponse struct {
	Error *APIError `json:"error"`
}

// NewAPIError creates a new API error
func NewAPIError(code ErrorCode, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// WithDetails adds details to the error
func (e *APIError) WithDetails(details map[string]interface{}) *APIError {
	e.Details = details
	return e
}

// WithRequestID adds request ID to the error
func (e *APIError) WithRequestID(requestID string) *APIError {
	e.RequestID = requestID
	return e
}

// GetHTTPStatusCode returns the appropriate HTTP status code for an error code
func GetHTTPStatusCode(code ErrorCode) int {
	switch code {
	// 400 Bad Request - Validation errors
	case ErrorCodeInvalidRequest, ErrorCodeMissingField, ErrorCodeInvalidCurrency,
		ErrorCodeNegativeAmount, ErrorCodePrecisionExceeded, ErrorCodeUnbalancedTransaction,
		ErrorCodeInvalidState, ErrorCodeInvalidFormat:
		return http.StatusBadRequest

	// 401 Unauthorized
	case ErrorCodeUnauthorized, ErrorCodeInvalidCredentials:
		return http.StatusUnauthorized

	// 403 Forbidden
	case ErrorCodeForbidden:
		return http.StatusForbidden

	// 404 Not Found
	case ErrorCodeAccountNotFound, ErrorCodeTransactionNotFound:
		return http.StatusNotFound

	// 409 Conflict - Idempotency conflicts
	case ErrorCodeIdempotencyConflict, ErrorCodeDuplicateKey:
		return http.StatusConflict

	// 422 Unprocessable Entity - Business rule violations
	case ErrorCodeInsufficientBalance, ErrorCodeNegativeBalancePolicy,
		ErrorCodeInvalidReversal, ErrorCodeUnsupportedCurrency:
		return http.StatusUnprocessableEntity

	// 500 Internal Server Error - System errors
	case ErrorCodeDatabaseError, ErrorCodeEventEmissionError,
		ErrorCodeInternalError, ErrorCodeServiceUnavailable:
		return http.StatusInternalServerError

	default:
		return http.StatusInternalServerError
	}
}

// SendErrorResponse sends a standardized error response
func SendErrorResponse(c *gin.Context, apiError *APIError) {
	// Get request ID from context if available
	if requestID, exists := c.Get("request_id"); exists {
		if reqIDStr, ok := requestID.(string); ok {
			apiError.RequestID = reqIDStr
		}
	}

	statusCode := GetHTTPStatusCode(apiError.Code)

	response := ErrorResponse{
		Error: apiError,
	}

	c.JSON(statusCode, response)
}

// ValidationError creates a validation error
func ValidationError(message string) *APIError {
	return NewAPIError(ErrorCodeInvalidRequest, message)
}

// InsufficientBalanceError creates an insufficient balance error
func InsufficientBalanceError(accountID, currency string, required, available string) *APIError {
	return NewAPIError(ErrorCodeInsufficientBalance,
		fmt.Sprintf("Account %s has insufficient balance in %s", accountID, currency)).
		WithDetails(map[string]interface{}{
			"account_id":        accountID,
			"currency":          currency,
			"required_amount":   required,
			"available_balance": available,
		})
}

// AccountNotFoundError creates an account not found error
func AccountNotFoundError(accountID string) *APIError {
	return NewAPIError(ErrorCodeAccountNotFound,
		fmt.Sprintf("Account not found: %s", accountID)).
		WithDetails(map[string]interface{}{
			"account_id": accountID,
		})
}

// TransactionNotFoundError creates a transaction not found error
func TransactionNotFoundError(transactionID string) *APIError {
	return NewAPIError(ErrorCodeTransactionNotFound,
		fmt.Sprintf("Transaction not found: %s", transactionID)).
		WithDetails(map[string]interface{}{
			"transaction_id": transactionID,
		})
}

// InvalidCurrencyError creates an invalid currency error
func InvalidCurrencyError(currency string) *APIError {
	return NewAPIError(ErrorCodeInvalidCurrency,
		fmt.Sprintf("Invalid or unsupported currency: %s", currency)).
		WithDetails(map[string]interface{}{
			"currency": currency,
		})
}

// UnbalancedTransactionError creates an unbalanced transaction error
func UnbalancedTransactionError(debits, credits string) *APIError {
	return NewAPIError(ErrorCodeUnbalancedTransaction,
		fmt.Sprintf("Transaction is not balanced: debits (%s) must equal credits (%s)", debits, credits)).
		WithDetails(map[string]interface{}{
			"total_debits":  debits,
			"total_credits": credits,
		})
}

// IdempotencyConflictError creates an idempotency conflict error
func IdempotencyConflictError(key string) *APIError {
	return NewAPIError(ErrorCodeIdempotencyConflict,
		fmt.Sprintf("Idempotency key conflict: %s", key)).
		WithDetails(map[string]interface{}{
			"idempotency_key": key,
		})
}

// DatabaseError creates a database error
func DatabaseError(message string) *APIError {
	return NewAPIError(ErrorCodeDatabaseError,
		fmt.Sprintf("Database operation failed: %s", message))
}

// InternalError creates an internal server error
func InternalError(message string) *APIError {
	return NewAPIError(ErrorCodeInternalError,
		fmt.Sprintf("Internal server error: %s", message))
}

// ConvertToAPIError converts various error types to standardized API errors
func ConvertToAPIError(err error) *APIError {
	if err == nil {
		return InternalError("Unknown error occurred")
	}

	// Handle domain validation errors
	if validationErr, ok := err.(domain.ValidationError); ok {
		return convertValidationError(validationErr)
	}

	// Handle insufficient balance errors
	if insufficientErr, ok := err.(*service.InsufficientBalanceError); ok {
		return InsufficientBalanceError(
			insufficientErr.AccountID,
			insufficientErr.Currency,
			insufficientErr.RequiredAmount.String(),
			insufficientErr.AvailableBalance.String(),
		)
	}

	// Handle common error patterns by message content
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)

	// Account not found errors
	if strings.Contains(errMsgLower, "account not found") ||
		strings.Contains(errMsgLower, "account") && strings.Contains(errMsgLower, "not found") {
		return extractAccountNotFoundError(errMsg)
	}

	// Transaction not found errors
	if strings.Contains(errMsgLower, "transaction not found") ||
		strings.Contains(errMsgLower, "transaction") && strings.Contains(errMsgLower, "not found") {
		return extractTransactionNotFoundError(errMsg)
	}

	// Currency validation errors
	if strings.Contains(errMsgLower, "currency") &&
		(strings.Contains(errMsgLower, "invalid") || strings.Contains(errMsgLower, "unsupported")) {
		return extractCurrencyError(errMsg)
	}

	// Insufficient balance errors (string-based detection)
	if strings.Contains(errMsgLower, "insufficient balance") {
		return extractInsufficientBalanceError(errMsg)
	}

	// Unbalanced transaction errors
	if strings.Contains(errMsgLower, "not balanced") ||
		strings.Contains(errMsgLower, "debits") && strings.Contains(errMsgLower, "credits") {
		return extractUnbalancedTransactionError(errMsg)
	}

	// Validation errors (general)
	if strings.Contains(errMsgLower, "validation failed") ||
		strings.Contains(errMsgLower, "invalid") {
		return ValidationError(errMsg)
	}

	// Database errors
	if strings.Contains(errMsgLower, "database") ||
		strings.Contains(errMsgLower, "connection") ||
		strings.Contains(errMsgLower, "sql") {
		return DatabaseError("Database operation failed")
	}

	// Event emission errors
	if strings.Contains(errMsgLower, "event") &&
		(strings.Contains(errMsgLower, "failed") || strings.Contains(errMsgLower, "emit")) {
		return NewAPIError(ErrorCodeEventEmissionError, "Event emission failed")
	}

	// Idempotency conflicts
	if strings.Contains(errMsgLower, "idempotency") &&
		(strings.Contains(errMsgLower, "conflict") || strings.Contains(errMsgLower, "exists")) {
		return extractIdempotencyConflictError(errMsg)
	}

	// Default to internal error
	return InternalError(errMsg)
}

// convertValidationError converts domain validation errors to API errors
func convertValidationError(validationErr domain.ValidationError) *APIError {
	field := validationErr.Field
	message := validationErr.Message

	// Determine specific error code based on field and message
	msgLower := strings.ToLower(message)

	if strings.Contains(msgLower, "currency") {
		if strings.Contains(msgLower, "invalid") || strings.Contains(msgLower, "unsupported") {
			return InvalidCurrencyError(extractValueFromMessage(message))
		}
	}

	if strings.Contains(msgLower, "amount") && strings.Contains(msgLower, "negative") {
		return NewAPIError(ErrorCodeNegativeAmount, message).
			WithDetails(map[string]interface{}{"field": field})
	}

	if strings.Contains(msgLower, "precision") || strings.Contains(msgLower, "decimal places") {
		return NewAPIError(ErrorCodePrecisionExceeded, message).
			WithDetails(map[string]interface{}{"field": field})
	}

	if strings.Contains(msgLower, "balanced") ||
		(strings.Contains(msgLower, "debits") && strings.Contains(msgLower, "credits")) {
		return extractUnbalancedTransactionError(message)
	}

	if strings.Contains(msgLower, "required") || strings.Contains(msgLower, "missing") {
		return NewAPIError(ErrorCodeMissingField, message).
			WithDetails(map[string]interface{}{"field": field})
	}

	if strings.Contains(msgLower, "state") && strings.Contains(msgLower, "invalid") {
		return NewAPIError(ErrorCodeInvalidState, message).
			WithDetails(map[string]interface{}{"field": field})
	}

	// Default validation error
	return ValidationError(message).
		WithDetails(map[string]interface{}{"field": field})
}

// Helper functions to extract specific error information

func extractAccountNotFoundError(errMsg string) *APIError {
	// Try to extract account ID from message
	accountID := extractValueFromMessage(errMsg)
	if accountID == "" {
		accountID = "unknown"
	}
	return AccountNotFoundError(accountID)
}

func extractTransactionNotFoundError(errMsg string) *APIError {
	// Try to extract transaction ID from message
	transactionID := extractValueFromMessage(errMsg)
	if transactionID == "" {
		transactionID = "unknown"
	}
	return TransactionNotFoundError(transactionID)
}

func extractCurrencyError(errMsg string) *APIError {
	// Try to extract currency from message
	currency := extractValueFromMessage(errMsg)
	if currency == "" {
		currency = "unknown"
	}
	return InvalidCurrencyError(currency)
}

func extractInsufficientBalanceError(errMsg string) *APIError {
	// Create a generic insufficient balance error
	// The service layer should provide structured errors for better extraction
	return NewAPIError(ErrorCodeInsufficientBalance, errMsg)
}

func extractUnbalancedTransactionError(errMsg string) *APIError {
	// Try to extract debit and credit amounts from message
	// Look for patterns like "debits (100.00) must equal credits (200.00)"
	return NewAPIError(ErrorCodeUnbalancedTransaction, errMsg)
}

func extractIdempotencyConflictError(errMsg string) *APIError {
	// Try to extract idempotency key from message
	key := extractValueFromMessage(errMsg)
	if key == "" {
		key = "unknown"
	}
	return IdempotencyConflictError(key)
}

// extractValueFromMessage attempts to extract a value (ID, currency, etc.) from an error message
func extractValueFromMessage(message string) string {
	// Simple extraction - look for common patterns
	// This could be enhanced with regex for more precise extraction

	// Look for UUID patterns (account IDs, transaction IDs)
	words := strings.Fields(message)
	for _, word := range words {
		// Remove common punctuation
		word = strings.Trim(word, ",:;.()")

		// Check if it looks like a UUID or identifier
		if len(word) > 8 && (strings.Contains(word, "-") || len(word) == 36) {
			return word
		}

		// Check if it's a 3-letter currency code
		if len(word) == 3 && strings.ToUpper(word) == word {
			return word
		}
	}

	return ""
}
