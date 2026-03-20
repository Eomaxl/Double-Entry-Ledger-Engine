package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/database"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/events"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/postgres"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/tracing"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service"
	"go.uber.org/zap"
)

type App struct {
	server          *http.Server
	logger          *zap.Logger
	db              *database.PostgresDB
	eventPublisher  events.EventPublisher
	tracingShutdown func(context.Context) error
	lifecycle       *Lifecycle
	shutdownTimeout time.Duration
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	appMetrics := metrics.NewMetrics()
	lifecycle := NewLifecycle()

	tracingShutdown, err := tracing.InitTracing(tracing.TracingConfig{
		Enabled:     cfg.Tracing.Enabled,
		ServiceName: cfg.Tracing.ServiceName,
		Endpoint:    cfg.Tracing.Endpoint,
		Environment: cfg.Tracing.Environment,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize tracing: %w", err)
	}

	db, err := database.NewPostgresDB(cfg.Database, logger, appMetrics)
	if err != nil {
		_ = tracingShutdown(context.Background())
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	if err := database.RunMigrations(cfg.Database, logger); err != nil {
		db.Close()
		_ = tracingShutdown(context.Background())
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	eventPublisher, err := newEventPublisher(cfg, logger)
	if err != nil {
		db.Close()
		_ = tracingShutdown(context.Background())
		return nil, fmt.Errorf("initialize event publisher: %w", err)
	}

	accountRepo := postgres.NewPostgresAccountRepository(db.Pool, logger)
	idempotencyRepo := postgres.NewPostgresIdempotencyRepository(db.Pool, logger)
	ledgerRepo := postgres.NewPostgresLedgerRepository()

	validator := domain.NewTransactionValidator(cfg.Currencies.Supported)
	balanceCalculator := service.NewPostgresBalanceCalculator(db.Pool, logger, appMetrics)
	transactionProcessor := service.NewPostgresTransactionProcessor(
		db.Pool,
		validator,
		accountRepo,
		idempotencyRepo,
		balanceCalculator,
		ledgerRepo,
		eventPublisher,
		cfg.Idempotency.RetentionPeriod,
		logger,
		appMetrics,
	)
	queryService := service.NewPostgresQueryService(db.Pool, ledgerRepo, logger)

	dbProbe := database.NewPoolProbe(db.Pool)
	router := api.NewRouter(
		dbProbe,
		transactionProcessor,
		accountRepo,
		balanceCalculator,
		queryService,
		cfg,
		appMetrics,
		logger,
		lifecycle,
	)

	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           router.GetEngine(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return &App{
		server:          server,
		logger:          logger,
		db:              db,
		eventPublisher:  eventPublisher,
		tracingShutdown: tracingShutdown,
		lifecycle:       lifecycle,
		shutdownTimeout: cfg.Server.ShutdownTimeout,
	}, nil
}

func (a *App) Start() error {
	a.lifecycle.SetReady(true)
	a.logger.Info("starting HTTP server", zap.String("address", a.server.Addr))
	err := a.server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	var shutdownErr error
	a.lifecycle.StartShutdown()

	if err := a.server.Shutdown(ctx); err != nil {
		a.logger.Error("server shutdown failed", zap.Error(err))
		shutdownErr = err
	}

	if a.eventPublisher != nil {
		if err := a.eventPublisher.Close(); err != nil {
			a.logger.Error("event publisher shutdown failed", zap.Error(err))
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}

	if a.db != nil {
		a.db.Close()
	}

	if a.tracingShutdown != nil {
		if err := a.tracingShutdown(ctx); err != nil {
			a.logger.Error("tracing shutdown failed", zap.Error(err))
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}

	return shutdownErr
}

func (a *App) ShutdownTimeout() time.Duration {
	return a.shutdownTimeout
}

func newEventPublisher(cfg *config.Config, logger *zap.Logger) (events.EventPublisher, error) {
	if !cfg.NATS.Enabled {
		logger.Warn("event publishing disabled; using no-op publisher")
		return events.NewNoOpEventPublisher(), nil
	}

	return nil, fmt.Errorf("NATS event publisher is enabled but no concrete publisher is implemented")
}
