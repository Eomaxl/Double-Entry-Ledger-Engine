package handlers

import (
	"net/http"
	"strconv"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/errors"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AccountHandler handles HTTP requests for account operations
type AccountHandler struct {
	accountRepo repository.AccountRepository
	metrics     *metrics.Metrics
	logger      *zap.Logger
}

// NewAccountHandler creates a new account handler
func NewAccountHandler(
	accountRepo repository.AccountRepository,
	metrics *metrics.Metrics,
	logger *zap.Logger,
) *AccountHandler {
	return &AccountHandler{
		accountRepo: accountRepo,
		metrics:     metrics,
		logger:      logger,
	}
}

// CreateAccountRequest represents the request body for creating an account
type CreateAccountRequest struct {
	AccountType domain.AccountType      `json:"account_type" binding:"required,oneof=asset liability equity revenue expense"`
	Currencies  []AccountCurrencyConfig `json:"currencies" binding:"required,min=1,dive"`
	Metadata    map[string]interface{}  `json:"metadata,omitempty"`
}

// AccountCurrencyConfig represents currency configuration for an account
type AccountCurrencyConfig struct {
	CurrencyCode  string `json:"currency_code" binding:"required,len=3,alpha"`
	AllowNegative bool   `json:"allow_negative"`
}

// UpdateAccountMetadataRequest represents the request body for updating account metadata
type UpdateAccountMetadataRequest struct {
	Metadata map[string]interface{} `json:"metadata" binding:"required"`
}

// CreateAccount handles POST /v1/accounts - Create account
func (h *AccountHandler) CreateAccount(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid account creation request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Convert to domain request
	currencies := make([]domain.CurrencyConfig, len(req.Currencies))
	for i, curr := range req.Currencies {
		currencies[i] = domain.CurrencyConfig{
			CurrencyCode:  curr.CurrencyCode,
			AllowNegative: curr.AllowNegative,
		}
	}

	domainReq := domain.CreateAccountRequest{
		AccountType: req.AccountType,
		Currencies:  currencies,
		Metadata:    req.Metadata,
	}

	// Create account
	account, err := h.accountRepo.CreateAccount(c.Request.Context(), domainReq)
	if err != nil {
		h.handleAccountError(c, err)
		return
	}

	h.logger.Info("account created successfully", zap.String("account_id", account.AccountID.String()))

	c.JSON(http.StatusCreated, account)
}

// GetAccount handles GET /v1/accounts/:id - Get account details
func (h *AccountHandler) GetAccount(c *gin.Context) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Account ID is required",
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

	account, err := h.accountRepo.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		h.handleAccountError(c, err)
		return
	}

	c.JSON(http.StatusOK, account)
}

// ListAccounts handles GET /v1/accounts - List accounts
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	filter := domain.AccountFilter{
		Limit:  100, // Default limit
		Offset: 0,   // Default offset
	}

	// Parse query parameters
	if currencyCode := c.Query("currency_code"); currencyCode != "" {
		filter.CurrencyCode = &currencyCode
	}

	if accountType := c.Query("account_type"); accountType != "" {
		accType := domain.AccountType(accountType)
		if accType.IsValid() {
			filter.AccountType = &accType
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid account_type parameter",
				"details": "Account type must be one of: asset, liability, equity, revenue, expense",
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

	accounts, err := h.accountRepo.ListAccounts(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list accounts", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve accounts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"count":    len(accounts),
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

// UpdateAccountMetadata handles PATCH /v1/accounts/:id/metadata - Update account metadata
func (h *AccountHandler) UpdateAccountMetadata(c *gin.Context) {
	accountIDStr := c.Param("id")
	if accountIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Account ID is required",
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

	var req UpdateAccountMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("invalid metadata update request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	err = h.accountRepo.UpdateAccountMetadata(c.Request.Context(), accountID, req.Metadata)
	if err != nil {
		h.handleAccountError(c, err)
		return
	}

	h.logger.Info("account metadata updated successfully", zap.String("account_id", accountID.String()))

	c.JSON(http.StatusOK, gin.H{
		"message":    "Account metadata updated successfully",
		"account_id": accountID.String(),
	})
}

// handleAccountError handles account operation errors
func (h *AccountHandler) handleAccountError(c *gin.Context, err error) {
	// Convert error to standardized API error and send response
	apiError := errors.ConvertToAPIError(err)
	errors.SendErrorResponse(c, apiError)
}
