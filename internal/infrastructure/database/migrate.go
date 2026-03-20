package database

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"
)

// RunMigrations applies pending database migrations
func RunMigrations(cfg config.DatabaseConfig, logger *zap.Logger) error {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database, cfg.SSLMode,
	)

	// Get the absolute path to migrations directory
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	migrationsPath := filepath.Join(projectRoot, "migrations")
	migrationsURL := fmt.Sprintf("file://%s", migrationsPath)

	m, err := migrate.New(
		migrationsURL,
		connString,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}
	defer m.Close()

	// Get current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	if dirty {
		return fmt.Errorf("database is in dirty state at version %d, manual intervention required", version)
	}

	logger.Info("current database schema version", zap.Uint("version", version))

	// Apply migrations
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			logger.Info("database schema is up to date")
			return nil
		}
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Get new version
	newVersion, _, err := m.Version()
	if err != nil {
		return fmt.Errorf("failed to get new migration version: %w", err)
	}

	logger.Info("database migrations applied successfully", zap.Uint("new_version", newVersion))
	return nil
}
