package middleware

import (
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/errors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// API Key authentication
	APIKeys map[string]APIKeyInfo `json:"api_keys"`

	// JWT authentication
	JWTSecret     string        `json:"jwt_secret"`
	JWTExpiration time.Duration `json:"jwt_expiration"`

	// Feature flags
	EnableAPIKey bool `json:"enable_api_key"`
	EnableJWT    bool `json:"enable_jwt"`
}

// APIKeyInfo holds information about an API key
type APIKeyInfo struct {
	Name        string            `json:"name"`
	Permissions map[string]bool   `json:"permissions"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Permission constants
const (
	PermissionTransactionPost    = "transaction:post"
	PermissionTransactionRead    = "transaction:read"
	PermissionTransactionSettle  = "transaction:settle"
	PermissionTransactionCancel  = "transaction:cancel"
	PermissionTransactionReverse = "transaction:reverse"
	PermissionAccountCreate      = "account:create"
	PermissionAccountRead        = "account:read"
	PermissionAccountUpdate      = "account:update"
	PermissionBalanceRead        = "balance:read"
)

// AuthContext holds authentication information
type AuthContext struct {
	CallerID        string            `json:"caller_id"`
	CallerType      string            `json:"caller_type"` // "api_key" or "jwt"
	Permissions     map[string]bool   `json:"permissions"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	AuthenticatedAt time.Time         `json:"authenticated_at"`
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	CallerID    string            `json:"caller_id"`
	Permissions map[string]bool   `json:"permissions"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	jwt.RegisteredClaims
}

// Authentication creates a middleware that handles API key and JWT authentication
func Authentication(config AuthConfig, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authentication for health endpoints
		path := routePath(c)
		if isHealthEndpoint(path) {
			c.Next()
			return
		}

		apiKeyHeader := strings.TrimSpace(c.GetHeader("X-API-Key"))
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if apiKeyHeader == "" && authHeader == "" {
			logger.Warn("missing authorization header",
				zap.String("path", path),
				zap.String("method", c.Request.Method),
				zap.String("client_ip", c.ClientIP()))

			logAccessAttempt(logger, c, "", "missing_auth", false)
			apiError := errors.NewAPIError(errors.ErrorCodeUnauthorized, "authentication credentials are required")
			errors.SendErrorResponse(c, apiError)
			c.Abort()
			return
		}

		var authCtx *AuthContext
		var err error

		switch {
		case apiKeyHeader != "":
			if !config.EnableAPIKey {
				err = fmt.Errorf("API key authentication is disabled")
				break
			}
			authCtx, err = authenticateAPIKey(apiKeyHeader, config.APIKeys, logger)
		case strings.HasPrefix(authHeader, "Bearer "):
			if !config.EnableJWT {
				err = fmt.Errorf("JWT authentication is disabled")
				break
			}
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				err = fmt.Errorf("empty bearer token")
				break
			}
			authCtx, err = authenticateJWT(token, config.JWTSecret, logger)
		default:
			err = fmt.Errorf("unsupported authentication scheme")
		}

		if err != nil {
			logger.Warn("authentication failed",
				zap.Error(err),
				zap.String("path", path),
				zap.String("method", c.Request.Method),
				zap.String("client_ip", c.ClientIP()))

			logAccessAttempt(logger, c, "", "auth_failed", false)
			apiError := errors.NewAPIError(errors.ErrorCodeInvalidCredentials, "Invalid authentication credentials")
			errors.SendErrorResponse(c, apiError)
			c.Abort()
			return
		}

		// Store auth context for use by authorization middleware and handlers
		c.Set("auth_context", authCtx)

		// Log successful authentication
		logAccessAttempt(logger, c, authCtx.CallerID, "authenticated", true)

		logger.Debug("authentication successful",
			zap.String("caller_id", authCtx.CallerID),
			zap.String("caller_type", authCtx.CallerType),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method))

		c.Next()
	}
}

// Authorization creates a middleware that checks permissions for specific operations
func Authorization(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip authorization for health endpoints
		path := routePath(c)
		if isHealthEndpoint(path) {
			c.Next()
			return
		}

		// Get auth context from previous middleware
		authCtxInterface, exists := c.Get("auth_context")
		if !exists {
			logger.Error("authorization middleware called without authentication context")
			apiError := errors.NewAPIError(errors.ErrorCodeInternalError, "Authentication context not found")
			errors.SendErrorResponse(c, apiError)
			c.Abort()
			return
		}

		authCtx, ok := authCtxInterface.(*AuthContext)
		if !ok {
			logger.Error("invalid authentication context type")
			apiError := errors.NewAPIError(errors.ErrorCodeInternalError, "Invalid authentication context")
			errors.SendErrorResponse(c, apiError)
			c.Abort()
			return
		}

		// Determine required permission based on endpoint and method
		requiredPermission := getRequiredPermission(c.Request.Method, path)
		if requiredPermission == "" {
			// No specific permission required, allow access
			c.Next()
			return
		}

		// Check if caller has required permission
		if !authCtx.Permissions[requiredPermission] {
			logger.Warn("authorization failed - insufficient permissions",
				zap.String("caller_id", authCtx.CallerID),
				zap.String("required_permission", requiredPermission),
				zap.String("path", path),
				zap.String("method", c.Request.Method))

			logAccessAttempt(logger, c, authCtx.CallerID, "authorization_failed", false)
			apiError := errors.NewAPIError(errors.ErrorCodeForbidden,
				fmt.Sprintf("Insufficient permissions. Required: %s", requiredPermission))
			errors.SendErrorResponse(c, apiError)
			c.Abort()
			return
		}

		// Log successful authorization
		logAccessAttempt(logger, c, authCtx.CallerID, "authorized", true)

		logger.Debug("authorization successful",
			zap.String("caller_id", authCtx.CallerID),
			zap.String("permission", requiredPermission),
			zap.String("path", path),
			zap.String("method", c.Request.Method))

		c.Next()
	}
}

// authenticateAPIKey validates an API key and returns auth context
func authenticateAPIKey(token string, apiKeys map[string]APIKeyInfo, logger *zap.Logger) (*AuthContext, error) {
	// Use constant-time comparison to prevent timing attacks
	for key, keyInfo := range apiKeys {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1 {
			// Check if key is expired
			if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
				return nil, fmt.Errorf("API key expired")
			}

			return &AuthContext{
				CallerID:        keyInfo.Name,
				CallerType:      "api_key",
				Permissions:     keyInfo.Permissions,
				Metadata:        keyInfo.Metadata,
				AuthenticatedAt: time.Now(),
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid API key")
}

// authenticateJWT validates a JWT token and returns auth context
func authenticateJWT(tokenString, secret string, logger *zap.Logger) (*AuthContext, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid JWT claims")
	}

	// Check if token is expired
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("JWT token expired")
	}

	return &AuthContext{
		CallerID:        claims.CallerID,
		CallerType:      "jwt",
		Permissions:     claims.Permissions,
		Metadata:        claims.Metadata,
		AuthenticatedAt: time.Now(),
	}, nil
}

// getRequiredPermission determines the required permission for an endpoint
func getRequiredPermission(method, path string) string {
	// Transaction endpoints
	if strings.HasPrefix(path, "/v1/transactions") {
		switch method {
		case "POST":
			if strings.Contains(path, "/settle") {
				return PermissionTransactionSettle
			} else if strings.Contains(path, "/cancel") {
				return PermissionTransactionCancel
			} else if strings.Contains(path, "/reverse") {
				return PermissionTransactionReverse
			} else {
				return PermissionTransactionPost
			}
		case "GET":
			return PermissionTransactionRead
		}
	}

	// Account endpoints
	if strings.HasPrefix(path, "/v1/accounts") {
		// Balance endpoints (nested under accounts)
		if strings.Contains(path, "/balances") {
			return PermissionBalanceRead
		}

		switch method {
		case "POST":
			return PermissionAccountCreate
		case "GET":
			return PermissionAccountRead
		case "PATCH":
			return PermissionAccountUpdate
		}
	}

	// No specific permission required
	return ""
}

// isHealthEndpoint checks if the path is a health/monitoring endpoint
func isHealthEndpoint(path string) bool {
	healthPaths := []string{"/health", "/metrics", "/livez", "/readyz", "/v1/system/info"}
	for _, healthPath := range healthPaths {
		if path == healthPath {
			return true
		}
	}
	return false
}

func routePath(c *gin.Context) string {
	if fullPath := c.FullPath(); fullPath != "" {
		return fullPath
	}
	return c.Request.URL.Path
}

// logAccessAttempt logs all access attempts with caller identity
func logAccessAttempt(logger *zap.Logger, c *gin.Context, callerID, result string, success bool) {
	fields := []zap.Field{
		zap.String("caller_id", callerID),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.Request.UserAgent()),
		zap.String("result", result),
		zap.Bool("success", success),
		zap.Time("timestamp", time.Now()),
	}

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		if reqIDStr, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", reqIDStr))
		}
	}

	if success {
		logger.Info("access attempt", fields...)
	} else {
		logger.Warn("access attempt failed", fields...)
	}
}

// GetAuthContext retrieves the authentication context from Gin context
func GetAuthContext(c *gin.Context) (*AuthContext, bool) {
	authCtxInterface, exists := c.Get("auth_context")
	if !exists {
		return nil, false
	}

	authCtx, ok := authCtxInterface.(*AuthContext)
	return authCtx, ok
}
