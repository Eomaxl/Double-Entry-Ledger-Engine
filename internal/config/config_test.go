package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Database:        "ledger",
			SSLMode:         "disable",
			MaxConnections:  10,
			MinConnections:  2,
			MaxConnLifetime: time.Hour,
			MaxConnIdletime: time.Minute,
		},
		Server: ServerConfig{
			Host:            "0.0.0.0",
			Port:            8080,
			RequestTimeout:  time.Second,
			ShutdownTimeout: time.Second,
			MaxBodyBytes:    1024,
		},
		Idempotency: IdempotencyConfig{RetentionPeriod: time.Hour},
		Performance: PerformanceConfig{
			MaxBatchSize:        100,
			BalanceQueryTimeout: time.Second,
			TransactionTimeout:  time.Second,
		},
		Currencies: CurrenciesConfig{Supported: []string{"USD", "EUR"}},
		Logging:    LoggingConfig{Level: "info", Format: "json"},
		Auth: AuthConfig{
			EnableAPIKey:  true,
			EnableJWT:     false,
			JWTExpiration: time.Hour,
			APIKeys: []APIKeyConfig{
				{Key: "123456789012", Name: "dev", Permissions: []string{"transaction:read"}},
			},
		},
		Tracing: TracingConfig{Enabled: false},
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestValidate_DuplicateCurrency(t *testing.T) {
	cfg := validConfig()
	cfg.Currencies.Supported = []string{"USD", "USD"}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate currency") {
		t.Fatalf("expected duplicate currency error, got %v", err)
	}
}

func TestValidate_InvalidSSLMode(t *testing.T) {
	cfg := validConfig()
	cfg.Database.SSLMode = "bad"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "DB_SSL_MODE") {
		t.Fatalf("expected ssl mode error, got %v", err)
	}
}

func TestValidate_InvalidAuthConfig(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.EnableAPIKey = false
	cfg.Auth.EnableJWT = false
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "at least one authentication method") {
		t.Fatalf("expected auth method error, got %v", err)
	}
}

func TestValidate_JWTSecretTooShort(t *testing.T) {
	cfg := validConfig()
	cfg.Auth.EnableAPIKey = false
	cfg.Auth.EnableJWT = true
	cfg.Auth.JWTSecret = "short"
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "at least 32 characters") {
		t.Fatalf("expected jwt secret length error, got %v", err)
	}
}
