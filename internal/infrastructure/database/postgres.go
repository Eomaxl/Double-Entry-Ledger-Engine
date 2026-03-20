package database

import (
	"context"
	"fmt"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PostgresDB struct {
	Pool    *pgxpool.Pool
	logger  *zap.Logger
	metrics *metrics.Metrics
}

func NewPostgresDB(cfg config.DatabaseConfig, logger *zap.Logger, metrics *metrics.Metrics) (*PostgresDB, error) {
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = int32(cfg.MaxConnections)
	poolConfig.MinConns = int32(cfg.MinConnections)
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdletime

	// Create connection pool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("database connection pool established",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("database", cfg.Database),
		zap.Int("max_connections", cfg.MaxConnections),
		zap.Int("min_connections", cfg.MinConnections),
	)

	db := &PostgresDB{
		Pool:    pool,
		logger:  logger,
		metrics: metrics,
	}

	// Set initial metrics
	if metrics != nil {
		metrics.DBConnectionsMax.Set(float64(cfg.MaxConnections))

		// Start metrics collection goroutine
		go db.collectMetrics()
	}

	return db, nil
}

func (db *PostgresDB) Close() {
	db.logger.Info("closing database connection pool")
	db.Pool.Close()
}

func (db *PostgresDB) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}

	return nil
}

func (db *PostgresDB) Stats() *pgxpool.Stat {
	return db.Pool.Stat()
}

func (db *PostgresDB) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second) // Collect metrics every 10 seconds
	defer ticker.Stop()

	for range ticker.C {
		if db.metrics == nil {
			return
		}

		stats := db.Pool.Stat()

		// Update connection pool metrics
		db.metrics.DBConnectionsActive.Set(float64(stats.AcquiredConns()))
		db.metrics.DBConnectionsIdle.Set(float64(stats.IdleConns()))

		// Log metrics periodically for debugging
		db.logger.Debug("database connection pool metrics",
			zap.Int32("active_connections", stats.AcquiredConns()),
			zap.Int32("idle_connections", stats.IdleConns()),
			zap.Int32("total_connections", stats.TotalConns()),
			zap.Int32("max_connections", stats.MaxConns()),
		)
	}
}
