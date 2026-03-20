package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Database    DatabaseConfig
	NATS        NATSConfig
	Server      ServerConfig
	Idempotency IdempotencyConfig
	Performance PerformanceConfig
	Currencies  CurrenciesConfig
	Logging     LoggingConfig
	Auth        AuthConfig
	Tracing     TracingConfig
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConnections  int
	MinConnections  int
	MaxConnLifetime time.Duration
	MaxConnIdletime time.Duration
}

type NATSConfig struct {
	Enabled bool
	URL     string
	Subject string
}

type ServerConfig struct {
	Host            string
	Port            int
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	MaxBodyBytes    int64
}

type IdempotencyConfig struct {
	RetentionPeriod time.Duration
}

type PerformanceConfig struct {
	MaxBatchSize           int
	BalanceQueryTimeout    time.Duration
	TransactionTimeout     time.Duration
	EnableBalanceSnapshots bool
}

type CurrenciesConfig struct {
	Supported []string
}

type LoggingConfig struct {
	Level  string
	Format string // json or console
}

type AuthConfig struct {
	EnableAPIKey  bool           `json:"enable_api_key"`
	EnableJWT     bool           `json:"enable_jwt"`
	JWTSecret     string         `json:"jwt_secret"`
	JWTExpiration time.Duration  `json:"jwt_expiration"`
	APIKeys       []APIKeyConfig `json:"api_keys"`
}

type TracingConfig struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name"`
	Endpoint    string `json:"endpoint"`
	Environment string `json:"environment"`
}

type APIKeyConfig struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Permissions []string          `json:"permissions"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func Load() (*Config, error) {
	cfg := &Config{
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", ""),
			Database:        getEnv("DB_NAME", "ledger"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxConnections:  getEnvAsInt("DB_MAX_CONNECTIONS", 25),
			MinConnections:  getEnvAsInt("DB_MIN_CONNECTIONS", 5),
			MaxConnLifetime: getEnvAsDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour),
			MaxConnIdletime: getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		},
		NATS: NATSConfig{
			Enabled: getEnvAsBool("NATS_ENABLED", false),
			URL:     getEnv("NATS_URL", "nats://localhost:4222"),
			Subject: getEnv("NATS_SUBJECT", "ledger.events"),
		},
		Server: ServerConfig{
			Host:            getEnv("SERVER_HOST", "0.0.0.0"),
			Port:            getEnvAsInt("SERVER_PORT", 8080),
			RequestTimeout:  getEnvAsDuration("SERVER_REQUEST_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("SERVER_SHUTDOWN_TIMEOUT", 30*time.Second),
			MaxBodyBytes:    getEnvAsInt64("SERVER_MAX_BODY_BYTES", 1024*1024),
		},
		Idempotency: IdempotencyConfig{
			RetentionPeriod: getEnvAsDuration("IDEMPOTENCY_RETENTION", 24*time.Hour),
		},
		Performance: PerformanceConfig{
			MaxBatchSize:           getEnvAsInt("MAX_BATCH_SIZE", 1000),
			BalanceQueryTimeout:    getEnvAsDuration("BALANCE_QUERY_TIMEOUT", 100*time.Millisecond),
			TransactionTimeout:     getEnvAsDuration("TRANSACTION_TIMEOUT", 5*time.Second),
			EnableBalanceSnapshots: getEnvAsBool("ENABLE_BALANCE_SNAPSHOTS", false),
		},
		Currencies: CurrenciesConfig{
			Supported: getEnvAsSlice("SUPPORTED_CURRENCIES", []string{"USD", "EUR", "GBP", "JPY", "CNY"}),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Auth: AuthConfig{
			EnableAPIKey:  getEnvAsBool("AUTH_ENABLE_API_KEY", true),
			EnableJWT:     getEnvAsBool("AUTH_ENABLE_JWT", false),
			JWTSecret:     getEnv("AUTH_JWT_SECRET", ""),
			JWTExpiration: getEnvAsDuration("AUTH_JWT_EXPIRATION", 24*time.Hour),
			APIKeys:       loadAPIKeysFromEnv(),
		},
		Tracing: TracingConfig{
			Enabled:     getEnvAsBool("TRACING_ENABLED", false),
			ServiceName: getEnv("TRACING_SERVICE_NAME", "double-entry-ledger-engine"),
			Endpoint:    getEnv("TRACING_ENDPOINT", "http://localhost:4318/v1/traces"),
			Environment: getEnv("TRACING_ENVIRONMENT", "development"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed : %w", err)
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	// Database validation
	if c.Database.Host == "" {
		return fmt.Errorf("DB_HOST is required")
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", c.Database.Port)
	}

	if c.Database.User == "" {
		return fmt.Errorf("DB_USER is required")
	}

	if c.Database.Database == "" {
		return fmt.Errorf("DB_NAME is required")
	}

	if c.Database.MaxConnections <= 0 {
		return fmt.Errorf("DB_MAX_CONNECTIONS must be positive, got %d", c.Database.MaxConnections)
	}

	if c.Database.MinConnections < 0 {
		return fmt.Errorf("DB_MIN_CONNECTIONS must be non-negative, got %d", c.Database.MinConnections)
	}

	if c.Database.MinConnections > c.Database.MaxConnections {
		return fmt.Errorf("DB_MIN_CONNECTIONS (%d) cannot exceed DB_MAX_CONNECTIONS (%d)",
			c.Database.MinConnections, c.Database.MaxConnections)
	}
	if c.Database.MaxConnLifetime <= 0 {
		return fmt.Errorf("DB_MAX_CONN_LIFETIME must be positive, got %v", c.Database.MaxConnLifetime)
	}
	if c.Database.MaxConnIdletime <= 0 {
		return fmt.Errorf("DB_MAX_CONN_IDLE_TIME must be positive, got %v", c.Database.MaxConnIdletime)
	}
	validSSLModes := map[string]bool{"disable": true, "require": true, "verify-ca": true, "verify-full": true}
	if !validSSLModes[c.Database.SSLMode] {
		return fmt.Errorf("DB_SSL_MODE must be one of: disable, require, verify-ca, verify-full; got '%s'", c.Database.SSLMode)
	}

	// NATS validation
	if c.NATS.Enabled {
		if c.NATS.URL == "" {
			return fmt.Errorf("NATS_URL is required when NATS is enabled")
		}
		if c.NATS.Subject == "" {
			return fmt.Errorf("NATS_SUBJECT is required when NATS is enabled")
		}
		if !strings.HasPrefix(c.NATS.URL, "nats://") {
			return fmt.Errorf("NATS_URL must start with 'nats://', got '%s'", c.NATS.URL)
		}
	}

	// Server validation
	if c.Server.Host == "" {
		return fmt.Errorf("SERVER_HOST is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Server.RequestTimeout <= 0 {
		return fmt.Errorf("SERVER_REQUEST_TIMEOUT must be positive, got %v", c.Server.RequestTimeout)
	}
	if c.Server.ShutdownTimeout <= 0 {
		return fmt.Errorf("SERVER_SHUTDOWN_TIMEOUT must be positive, got %v", c.Server.ShutdownTimeout)
	}
	if c.Server.MaxBodyBytes <= 0 {
		return fmt.Errorf("SERVER_MAX_BODY_BYTES must be positive, got %d", c.Server.MaxBodyBytes)
	}

	// Idempotency validation
	if c.Idempotency.RetentionPeriod <= 0 {
		return fmt.Errorf("IDEMPOTENCY_RETENTION must be positive, got %v", c.Idempotency.RetentionPeriod)
	}

	// Performance validation
	if c.Performance.MaxBatchSize <= 0 || c.Performance.MaxBatchSize > 10000 {
		return fmt.Errorf("MAX_BATCH_SIZE must be between 1 and 10000, got %d", c.Performance.MaxBatchSize)
	}
	if c.Performance.BalanceQueryTimeout <= 0 {
		return fmt.Errorf("BALANCE_QUERY_TIMEOUT must be positive, got %v", c.Performance.BalanceQueryTimeout)
	}
	if c.Performance.TransactionTimeout <= 0 {
		return fmt.Errorf("TRANSACTION_TIMEOUT must be positive, got %v", c.Performance.TransactionTimeout)
	}

	// Validate timeout ranges for reasonable values
	if c.Performance.BalanceQueryTimeout > 10*time.Second {
		return fmt.Errorf("BALANCE_QUERY_TIMEOUT should not exceed 10 seconds, got %v", c.Performance.BalanceQueryTimeout)
	}
	if c.Performance.TransactionTimeout > 5*time.Minute {
		return fmt.Errorf("TRANSACTION_TIMEOUT should not exceed 5 minutes, got %v", c.Performance.TransactionTimeout)
	}

	// Currencies validation
	if len(c.Currencies.Supported) == 0 {
		return fmt.Errorf("SUPPORTED_CURRENCIES must contain at least one currency")
	}
	validCurrencyPattern := `^[A-Z]{3}$`
	for _, currency := range c.Currencies.Supported {
		if len(currency) != 3 {
			return fmt.Errorf("invalid currency code '%s': must be 3 characters", currency)
		}
		// Validate currency code format (3 uppercase letters)
		matched, _ := regexp.MatchString(validCurrencyPattern, currency)
		if !matched {
			return fmt.Errorf("invalid currency code '%s': must be 3 uppercase letters", currency)
		}
	}

	// Check for duplicate currencies
	currencySet := make(map[string]bool)
	for _, currency := range c.Currencies.Supported {
		if currencySet[currency] {
			return fmt.Errorf("duplicate currency code '%s' in SUPPORTED_CURRENCIES", currency)
		}
		currencySet[currency] = true
	}

	// Logging validation
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	if !validLogLevels[strings.ToLower(c.Logging.Level)] {
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error, fatal; got '%s'", c.Logging.Level)
	}
	validLogFormats := map[string]bool{"json": true, "console": true}
	if !validLogFormats[strings.ToLower(c.Logging.Format)] {
		return fmt.Errorf("LOG_FORMAT must be one of: json, console; got '%s'", c.Logging.Format)
	}

	// Auth validation
	if !c.Auth.EnableAPIKey && !c.Auth.EnableJWT {
		return fmt.Errorf("at least one authentication method must be enabled (AUTH_ENABLE_API_KEY or AUTH_ENABLE_JWT)")
	}
	if c.Auth.EnableJWT && c.Auth.JWTSecret == "" {
		return fmt.Errorf("AUTH_JWT_SECRET is required when JWT authentication is enabled")
	}
	if c.Auth.EnableJWT && len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("AUTH_JWT_SECRET must be at least 32 characters when JWT authentication is enabled")
	}
	if c.Auth.JWTExpiration <= 0 {
		return fmt.Errorf("AUTH_JWT_EXPIRATION must be positive, got %v", c.Auth.JWTExpiration)
	}
	if c.Auth.EnableAPIKey && len(c.Auth.APIKeys) == 0 {
		return fmt.Errorf("at least one API key must be configured when API key authentication is enabled")
	}
	for i, apiKey := range c.Auth.APIKeys {
		if apiKey.Key == "" {
			return fmt.Errorf("API key at index %d has empty key", i)
		}
		if len(apiKey.Key) < 12 {
			return fmt.Errorf("API key '%s' must be at least 12 characters", apiKey.Name)
		}
		if apiKey.Name == "" {
			return fmt.Errorf("API key at index %d has empty name", i)
		}
		if len(apiKey.Permissions) == 0 {
			return fmt.Errorf("API key '%s' has no permissions configured", apiKey.Name)
		}
	}

	// Tracing validation
	if c.Tracing.Enabled {
		if c.Tracing.ServiceName == "" {
			return fmt.Errorf("TRACING_SERVICE_NAME is required when tracing is enabled")
		}
		if c.Tracing.Endpoint == "" {
			return fmt.Errorf("TRACING_ENDPOINT is required when tracing is enabled")
		}
		if c.Tracing.Environment == "" {
			return fmt.Errorf("TRACING_ENVIRONMENT is required when tracing is enabled")
		}
	}

	return nil

}

// Helper functions to read environment variables with default and validation

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

func getEnvAsInt64(key string, defaultValue int64) int64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return defaultValue
	}

	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultvalue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultvalue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultvalue
	}
	return value
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	parts := strings.Split(valueStr, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func loadAPIKeysFromEnv() []APIKeyConfig {
	keysStr := os.Getenv("AUTH_API_KEYS")
	if keysStr == "" {
		// Return default API key for development
		return []APIKeyConfig{
			{
				Key:  "dev-key-12345",
				Name: "development",
				Permissions: []string{
					"transaction:post", "transaction:read", "transaction:settle",
					"transaction:cancel", "transaction:reverse", "account:create",
					"account:read", "account:update", "balance:read",
				},
				Metadata: map[string]string{
					"environment": "development",
				},
			},
		}
	}

	var apiKeys []APIKeyConfig
	keyEntries := strings.Split(keysStr, ";")

	for _, entry := range keyEntries {
		parts := strings.Split(entry, ":")
		if len(parts) < 3 {
			continue // Skip invalid entries
		}

		key := parts[0]
		name := parts[1]
		permissions := strings.Split(parts[2], ",")

		apiKeys = append(apiKeys, APIKeyConfig{
			Key:         key,
			Name:        name,
			Permissions: permissions,
		})
	}

	return apiKeys
}
