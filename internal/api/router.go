package api

import (
	"time"

	apidocs "github.com/Eomaxl/double-entry-ledger-engine/internal/api/docs"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/handlers"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/middleware"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/database"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/repository"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/balance"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/query"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/transaction"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Router holds all dependencies for API routing
type Router struct {
	engine             *gin.Engine
	transactionHandler *handlers.TransactionHandler
	accountHandler     *handlers.AccountHandler
	balanceHandler     *handlers.BalanceHandler
	healthHandler      *handlers.HealthHandler
	metrics            *metrics.Metrics
	logger             *zap.Logger
	config             *config.Config
}

// NewRouter creates a new API router with all handlers and middleware
func NewRouter(
	dbProbe database.PoolProbe,
	transactionProcessor transaction.TransactionProcessor,
	accountRepo repository.AccountRepository,
	balanceCalculator balance.BalanceCalculator,
	queryService query.QueryService,
	config *config.Config,
	metrics *metrics.Metrics,
	logger *zap.Logger,
	probe handlers.Probe,
) *Router {
	// Set Gin mode based on environment
	if config.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Create handlers
	transactionHandler := handlers.NewTransactionHandler(transactionProcessor, queryService, metrics, logger)
	accountHandler := handlers.NewAccountHandler(accountRepo, metrics, logger)
	balanceHandler := handlers.NewBalanceHandler(balanceCalculator, metrics, logger)
	healthHandler := handlers.NewHealthHandler(dbProbe, config, logger, probe)

	router := &Router{
		engine:             engine,
		transactionHandler: transactionHandler,
		accountHandler:     accountHandler,
		balanceHandler:     balanceHandler,
		healthHandler:      healthHandler,
		metrics:            metrics,
		logger:             logger,
		config:             config,
	}

	router.setupMiddleware()
	router.setupRoutes()

	return router
}

// GetEngine returns the Gin engine for starting the server
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

// setupMiddleware configures global middleware
func (r *Router) setupMiddleware() {
	// Request ID middleware for tracing (must be first for error responses)
	r.engine.Use(middleware.RequestID())

	// Error handling and recovery middleware
	r.engine.Use(middleware.ErrorHandler(r.logger))

	// Request logging middleware
	r.engine.Use(middleware.RequestLogger(r.logger))
	r.engine.Use(middleware.CorrelationID())

	// Metrics middleware
	r.engine.Use(middleware.Metrics(r.metrics))

	// Request body size limit
	r.engine.Use(middleware.BodyLimit(r.config.Server.MaxBodyBytes))

	// CORS middleware (basic implementation)
	r.engine.Use(middleware.CORS())

	// Authentication middleware
	authConfig := r.buildAuthConfig()
	r.engine.Use(middleware.Authentication(authConfig, r.logger))

	// Authorization middleware
	r.engine.Use(middleware.Authorization(r.logger))

	// Tracing middleware
	r.engine.Use(middleware.Tracing())

	// Request timeout middleware
	r.engine.Use(middleware.Timeout(r.config.Server.RequestTimeout))
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Health and observability endpoints (no versioning)
	r.engine.GET("/health", r.healthHandler.HealthCheck)
	r.engine.GET("/livez", r.healthHandler.Liveness)
	r.engine.GET("/readyz", r.healthHandler.Readiness)
	r.engine.GET("/metrics", r.healthHandler.Metrics)
	apidocs.RegisterRoutes(r.engine)

	// API v1 routes
	v1 := r.engine.Group("/v1")
	{
		// System information
		v1.GET("/system/info", r.healthHandler.SystemInfo)

		// Transaction operations
		transactions := v1.Group("/transactions")
		{
			transactions.POST("", r.transactionHandler.PostTransaction)
			transactions.POST("/batch", r.transactionHandler.PostBatchTransactions)
			transactions.GET("/:id", r.transactionHandler.GetTransaction)
			transactions.POST("/:id/settle", r.transactionHandler.SettleTransaction)
			transactions.POST("/:id/cancel", r.transactionHandler.CancelTransaction)
			transactions.POST("/:id/reverse", r.transactionHandler.ReverseTransaction)
			transactions.GET("", r.transactionHandler.ListTransactions)
		}

		// Account operations
		accounts := v1.Group("/accounts")
		{
			accounts.POST("", r.accountHandler.CreateAccount)
			accounts.GET("/:id", r.accountHandler.GetAccount)
			accounts.GET("", r.accountHandler.ListAccounts)
			accounts.PATCH("/:id/metadata", r.accountHandler.UpdateAccountMetadata)

			// Balance operations (nested under accounts)
			accounts.GET("/:id/balances", r.balanceHandler.GetCurrentBalances)
			accounts.GET("/:id/balances/:currency", r.balanceHandler.GetCurrentBalance)
			accounts.GET("/:id/balances/:currency/history", r.balanceHandler.GetHistoricalBalance)
		}
	}

	// Catch-all for undefined routes
	r.engine.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error":   "Not Found",
			"message": "The requested endpoint does not exist",
			"path":    c.Request.URL.Path,
		})
	})
}

// buildAuthConfig converts config.AuthConfig to middleware.AuthConfig
func (r *Router) buildAuthConfig() middleware.AuthConfig {
	// Convert API keys from config format to middleware format
	apiKeys := make(map[string]middleware.APIKeyInfo)
	for _, keyConfig := range r.config.Auth.APIKeys {
		// Convert permissions slice to map
		permissions := make(map[string]bool)
		for _, perm := range keyConfig.Permissions {
			permissions[perm] = true
		}

		apiKeys[keyConfig.Key] = middleware.APIKeyInfo{
			Name:        keyConfig.Name,
			Permissions: permissions,
			CreatedAt:   time.Now(), // Use current time as creation time
			ExpiresAt:   keyConfig.ExpiresAt,
			Metadata:    keyConfig.Metadata,
		}
	}

	return middleware.AuthConfig{
		APIKeys:       apiKeys,
		JWTSecret:     r.config.Auth.JWTSecret,
		JWTExpiration: r.config.Auth.JWTExpiration,
		EnableAPIKey:  r.config.Auth.EnableAPIKey,
		EnableJWT:     r.config.Auth.EnableJWT,
	}
}
