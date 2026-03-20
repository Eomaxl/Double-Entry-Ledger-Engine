package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/errors"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// TransactionHandler handles HTTP requests for transaction operations
type TransactionHandler struct {
	processor service.TransactionProcessor
	query     service.QueryService
	metrics   *metrics.Metrics
	logger    *zap.Logger
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(
	processor service.TransactionProcessor,
	query service.QueryService,
	metrics *metrics.Metrics,
	logger *zap.Logger,
) *TransactionHandler {
	return &TransactionHandler{
		processor: processor,
		query:     query,
		metrics:   metrics,
		logger:    logger,
	}
}

// PostTransactionRequest represents the request body for posting a transaction
type PostTransactionRequest struct {
	IdempotencyKey *string                 `json:"idempotency_key,omitempty" binding:"omitempty,max=255"`
	State          domain.TransactionState `json:"state" binding:"required,oneof=pending settled"`
	Entries        []EntryRequest          `json:"entries" binding:"required,min=2,dive"`
	Metadata       map[string]interface{}  `json:"metadata,omitempty"`
}

// EntryRequest represents an entry in a transaction request
type EntryRequest struct {
	AccountID    string `json:"account_id" binding:"required,uuid"`
	CurrencyCode string `json:"currency_code" binding:"required,len=3,alpha"`
	Amount       string `json:"amount" binding:"required"`
	EntryType    string `json:"entry_type" binding:"required,oneof=debit credit"`
	Description  string `json:"description,omitempty" binding:"max=500"`
}

// BatchTransactionRequest represents a batch of transactions
type BatchTransactionRequest struct {
	Transactions []PostTransactionRequest `json:"transactions" binding:"required,min=1,max=1000,dive"`
}

// ReversalRequest represents a request to reverse a transaction
type ReversalRequest struct {
	IdempotencyKey *string                 `json:"idempotency_key,omitempty" binding:"omitempty,max=255"`
	State          domain.TransactionState `json:"state" binding:"required,oneof=pending settled"`
	Description    string                  `json:"description,omitempty" binding:"max=500"`
	Metadata       map[string]interface{}  `json:"metadata,omitempty"`
}

// PostTransaction handles POST /v1/transactions - Post new transaction
func (h *TransactionHandler) PostTransaction(c *gin.Context) {
	var req PostTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid transaction request", zap.Error(err))
		apiError := errors.ValidationError("Invalid request format: " + err.Error())
		errors.SendErrorResponse(c, apiError)
		return
	}

	// Convert to service request
	serviceReq, err := h.convertToServiceRequest(req)
	if err != nil {
		h.logger.Warn("failed to convert request", zap.Error(err))
		apiError := errors.ValidationError("Invalid request data: " + err.Error())
		errors.SendErrorResponse(c, apiError)
		return
	}

	// Process transaction
	txn, err := h.processor.PostTransaction(c.Request.Context(), *serviceReq)
	if err != nil {
		h.handleTransactionError(c, err)
		return
	}

	h.metrics.TransactionTotal.WithLabelValues(string(txn.State), "success").Inc()
	h.logger.Info("transaction posted successfully", zap.String("transaction_id", txn.TransactionID))

	c.JSON(http.StatusCreated, txn)
}

// PostBatchTransactions handles POST /v1/transactions/batch - Post batch transactions
func (h *TransactionHandler) PostBatchTransactions(c *gin.Context) {
	var req BatchTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid batch transaction request", zap.Error(err))
		apiError := errors.ValidationError("Invalid request format: " + err.Error())
		errors.SendErrorResponse(c, apiError)
		return
	}

	// Convert to service requests
	serviceReqs := make([]service.PostTransactionRequest, len(req.Transactions))
	for i, txnReq := range req.Transactions {
		serviceReq, err := h.convertToServiceRequest(txnReq)
		if err != nil {
			h.logger.Warn("failed to convert batch request", zap.Error(err), zap.Int("index", i))
			apiError := errors.ValidationError(fmt.Sprintf("Invalid request data at index %d: %s", i, err.Error()))
			errors.SendErrorResponse(c, apiError)
			return
		}
		serviceReqs[i] = *serviceReq
	}

	// Process batch
	txns, err := h.processor.PostBatch(c.Request.Context(), serviceReqs)
	if err != nil {
		h.handleTransactionError(c, err)
		return
	}

	h.metrics.BatchTransactionTotal.Inc()
	h.metrics.BatchTransactionSize.Observe(float64(len(txns)))
	h.logger.Info("batch transactions posted successfully", zap.Int("count", len(txns)))

	c.JSON(http.StatusCreated, gin.H{
		"transactions": txns,
		"count":        len(txns),
	})
}

// GetTransaction handles GET /v1/transactions/:id - Get transaction by ID
func (h *TransactionHandler) GetTransaction(c *gin.Context) {
	txnID := c.Param("id")
	if txnID == "" {
		apiError := errors.ValidationError("Transaction ID is required")
		errors.SendErrorResponse(c, apiError)
		return
	}

	txn, err := h.query.GetTransaction(c.Request.Context(), txnID)
	if err != nil {
		h.logger.Warn("failed to get transaction", zap.Error(err), zap.String("transaction_id", txnID))
		apiError := errors.ConvertToAPIError(err)
		errors.SendErrorResponse(c, apiError)
		return
	}

	c.JSON(http.StatusOK, txn)
}

// SettleTransaction handles POST /v1/transactions/:id/settle - Settle pending transaction
func (h *TransactionHandler) SettleTransaction(c *gin.Context) {
	txnID := c.Param("id")
	if txnID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Transaction ID is required",
		})
		return
	}

	txn, err := h.processor.SettlePendingTransaction(c.Request.Context(), txnID)
	if err != nil {
		h.handleTransactionError(c, err)
		return
	}

	h.metrics.TransactionTotal.WithLabelValues("settled", "success").Inc()
	h.logger.Info("transaction settled successfully", zap.String("transaction_id", txnID))

	c.JSON(http.StatusOK, txn)
}

// CancelTransaction handles POST /v1/transactions/:id/cancel - Cancel pending transaction
func (h *TransactionHandler) CancelTransaction(c *gin.Context) {
	txnID := c.Param("id")
	if txnID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Transaction ID is required",
		})
		return
	}

	txn, err := h.processor.CancelPendingTransaction(c.Request.Context(), txnID)
	if err != nil {
		h.handleTransactionError(c, err)
		return
	}

	h.metrics.TransactionTotal.WithLabelValues("cancelled", "success").Inc()
	h.logger.Info("transaction cancelled successfully", zap.String("transaction_id", txnID))

	c.JSON(http.StatusOK, txn)
}

// ReverseTransaction handles POST /v1/transactions/:id/reverse - Reverse transaction
func (h *TransactionHandler) ReverseTransaction(c *gin.Context) {
	originalTxnID := c.Param("id")
	if originalTxnID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Transaction ID is required",
		})
		return
	}

	var req ReversalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid reversal request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Convert to service request
	serviceReq := service.ReversalRequest{
		IdempotencyKey: req.IdempotencyKey,
		State:          req.State,
		Description:    req.Description,
		Metadata:       req.Metadata,
	}

	txn, err := h.processor.ReverseTransaction(c.Request.Context(), originalTxnID, serviceReq)
	if err != nil {
		h.handleTransactionError(c, err)
		return
	}

	h.metrics.TransactionTotal.WithLabelValues("reversal", "success").Inc()
	h.logger.Info("transaction reversed successfully",
		zap.String("reversal_transaction_id", txn.TransactionID),
		zap.String("original_transaction_id", originalTxnID))

	c.JSON(http.StatusCreated, txn)
}

// ListTransactions handles GET /v1/transactions - List transactions with filters
func (h *TransactionHandler) ListTransactions(c *gin.Context) {
	filter := service.TransactionFilter{
		Limit:  100, // Default limit
		Offset: 0,   // Default offset
	}

	// Parse query parameters
	if accountID := c.Query("account_id"); accountID != "" {
		filter.AccountID = &accountID
	}

	if currency := c.Query("currency"); currency != "" {
		filter.Currency = &currency
	}

	if state := c.Query("state"); state != "" {
		txnState := domain.TransactionState(state)
		if txnState.IsValid() {
			filter.State = &txnState
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid state parameter",
				"details": "State must be one of: pending, settled, cancelled",
			})
			return
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 1000 {
			filter.Limit = limit
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid limit parameter",
				"details": "Limit must be between 1 and 1000",
			})
			return
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid offset parameter",
				"details": "Offset must be non-negative",
			})
			return
		}
	}

	if orderBy := c.Query("order_by"); orderBy != "" {
		if orderBy == "timestamp_asc" || orderBy == "timestamp_desc" {
			filter.OrderBy = orderBy
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid order_by parameter",
				"details": "Order by must be 'timestamp_asc' or 'timestamp_desc'",
			})
			return
		}
	}

	// Parse metadata filters
	filter.Metadata = make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(key) > 9 && key[:9] == "metadata." {
			metaKey := key[9:]
			if len(values) > 0 {
				filter.Metadata[metaKey] = values[0]
			}
		}
	}

	result, err := h.query.ListTransactions(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve transactions",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// convertToServiceRequest converts HTTP request to service request
func (h *TransactionHandler) convertToServiceRequest(req PostTransactionRequest) (*service.PostTransactionRequest, error) {
	entries := make([]service.EntryRequest, len(req.Entries))

	for i, entry := range req.Entries {
		amount, err := decimal.NewFromString(entry.Amount)
		if err != nil {
			return nil, err
		}

		if amount.IsNegative() {
			return nil, fmt.Errorf("amount must be non-negative")
		}

		entryType := domain.EntryType(entry.EntryType)
		if !entryType.IsValid() {
			return nil, fmt.Errorf("invalid entry type: %s", entry.EntryType)
		}

		entries[i] = service.EntryRequest{
			AccountID:    entry.AccountID,
			CurrencyCode: entry.CurrencyCode,
			Amount:       amount,
			EntryType:    entryType,
			Description:  entry.Description,
		}
	}

	return &service.PostTransactionRequest{
		IdempotencyKey: req.IdempotencyKey,
		State:          req.State,
		Entries:        entries,
		Metadata:       req.Metadata,
	}, nil
}

// handleTransactionError handles transaction processing errors
func (h *TransactionHandler) handleTransactionError(c *gin.Context, err error) {
	h.metrics.TransactionErrors.WithLabelValues("processing").Inc()

	// Convert error to standardized API error and send response
	apiError := errors.ConvertToAPIError(err)
	errors.SendErrorResponse(c, apiError)
}
