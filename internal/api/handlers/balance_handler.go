package handlers

import (
	"net/http"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/errors"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// BalanceHandler handles HTTP requests for balance operations
type BalanceHandler struct {
	balanceCalculator service.BalanceCalculator
	metrics           *metrics.Metrics
	logger            *zap.Logger
}

// NewBalanceHandler creates a new balance handler
func NewBalanceHandler(
	balanceCalculator service.BalanceCalculator,
	metrics *metrics.Metrics,
	logger *zap.Logger,
) *BalanceHandler {
	return &BalanceHandler{
		balanceCalculator: balanceCalculator,
		metrics:           metrics,
		logger:            logger,
	}
}

// GetCurrentBalances handles GET /v1/accounts/:id/balances - Get current balances
func (h *BalanceHandler) GetCurrentBalances(c *gin.Context) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		apiError := errors.ValidationError("Account ID is required")
		errors.SendErrorResponse(c, apiError)
		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		apiError := errors.ValidationError("Invalid account ID format: " + err.Error())
		errors.SendErrorResponse(c, apiError)
		return
	}

	// Record metrics
	start := time.Now()
	defer func() {
		h.metrics.BalanceQueryDuration.Observe(time.Since(start).Seconds())
	}()

	balances, err := h.balanceCalculator.GetMultiCurrencyBalance(c.Request.Context(), accountID)
	if err != nil {
		h.handleBalanceError(c, err, "multi_currency")
		return
	}

	h.metrics.BalanceQueryTotal.WithLabelValues("multi_currency", "success").Inc()

	c.JSON(http.StatusOK, gin.H{
		"account_id": accountIDStr,
		"balances":   balances,
		"timestamp":  time.Now(),
	})
}

// GetCurrentBalance handles GET /v1/accounts/:id/balances/:currency - Get balance for currency
func (h *BalanceHandler) GetCurrentBalance(c *gin.Context) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Account ID is required",
		})
		return
	}

	currency := c.Param("currency")
	if currency == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Currency is required",
		})
		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid account ID format",
			"details": err.Error(),
		})
		return
	}

	// Validate currency format (3 letters)
	if len(currency) != 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid currency format",
			"details": "Currency must be a 3-letter code",
		})
		return
	}

	// Record metrics
	start := time.Now()
	defer func() {
		h.metrics.BalanceQueryDuration.Observe(time.Since(start).Seconds())
	}()

	balance, err := h.balanceCalculator.GetCurrentBalance(c.Request.Context(), accountID, currency)
	if err != nil {
		h.handleBalanceError(c, err, "current")
		return
	}

	h.metrics.BalanceQueryTotal.WithLabelValues("current", "success").Inc()

	c.JSON(http.StatusOK, balance)
}

// GetHistoricalBalance handles GET /v1/accounts/:id/balances/:currency/history - Get historical balance
func (h *BalanceHandler) GetHistoricalBalance(c *gin.Context) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Account ID is required",
		})
		return
	}

	currency := c.Param("currency")
	if currency == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Currency is required",
		})
		return
	}

	accountID, err := uuid.Parse(accountIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid account ID format",
			"details": err.Error(),
		})
		return
	}

	// Validate currency format (3 letters)
	if len(currency) != 3 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid currency format",
			"details": "Currency must be a 3-letter code",
		})
		return
	}

	// Parse as_of timestamp (required for historical queries)
	asOfStr := c.Query("as_of")
	if asOfStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "as_of parameter is required",
			"details": "Provide timestamp in RFC3339 format (e.g., 2024-01-15T10:30:00Z)",
		})
		return
	}

	asOf, err := time.Parse(time.RFC3339, asOfStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid as_of timestamp format",
			"details": "Timestamp must be in RFC3339 format (e.g., 2024-01-15T10:30:00Z)",
		})
		return
	}

	// Record metrics
	start := time.Now()
	defer func() {
		h.metrics.BalanceQueryDuration.Observe(time.Since(start).Seconds())
	}()

	balance, err := h.balanceCalculator.GetHistoricalBalance(c.Request.Context(), accountID, currency, asOf)
	if err != nil {
		h.handleBalanceError(c, err, "historical")
		return
	}

	h.metrics.BalanceQueryTotal.WithLabelValues("historical", "success").Inc()

	c.JSON(http.StatusOK, balance)
}

// handleBalanceError handles balance calculation errors
func (h *BalanceHandler) handleBalanceError(c *gin.Context, err error, queryType string) {
	h.metrics.BalanceQueryTotal.WithLabelValues(queryType, "error").Inc()

	// Convert error to standardized API error and send response
	apiError := errors.ConvertToAPIError(err)
	errors.SendErrorResponse(c, apiError)
}
