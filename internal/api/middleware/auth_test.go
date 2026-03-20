package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

func TestAuthentication_APIKeySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Authentication(AuthConfig{
		EnableAPIKey: true,
		APIKeys: map[string]APIKeyInfo{
			"k123456789012": {Name: "tester", Permissions: map[string]bool{PermissionTransactionRead: true}},
		},
	}, zap.NewNop()))
	r.GET("/v1/transactions", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	req.Header.Set("X-API-Key", "k123456789012")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
}

func TestAuthentication_MissingCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Authentication(AuthConfig{EnableAPIKey: true}, zap.NewNop()))
	r.GET("/v1/transactions", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/transactions", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}

func TestAuthentication_JWTDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Authentication(AuthConfig{EnableAPIKey: true, EnableJWT: false}, zap.NewNop()))
	r.GET("/v1/transactions", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", rec.Code)
	}
}

func TestAuthenticateJWT_ExpiredToken(t *testing.T) {
	secret := "12345678901234567890123456789012"
	claims := JWTClaims{
		CallerID:    "user1",
		Permissions: map[string]bool{PermissionTransactionRead: true},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = authenticateJWT(signed, secret, zap.NewNop())
	if err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestAuthorization_ForbiddenWithoutPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("auth_context", &AuthContext{
			CallerID:    "c1",
			Permissions: map[string]bool{PermissionTransactionPost: true},
		})
		c.Next()
	})
	r.Use(Authorization(zap.NewNop()))
	r.GET("/v1/transactions", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/transactions", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", rec.Code)
	}
}
