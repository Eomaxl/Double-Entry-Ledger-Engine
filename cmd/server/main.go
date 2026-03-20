package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/app"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/logging"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logger, err := logging.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting double-entry ledger engine",
		zap.String("version", "0.1.0"),
		zap.String("log_level", cfg.Logging.Level),
	)

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Fatal("failed to initialize application", zap.Error(err))
	}

	go func() {
		if err := application.Start(); err != nil {
			logger.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server gracefully")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), application.ShutdownTimeout())
	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		logger.Error("application shutdown completed with errors", zap.Error(err))
	}

	logger.Info("server stopped")
}
