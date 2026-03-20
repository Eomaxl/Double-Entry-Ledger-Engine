package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/api/handlers"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/config"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/balance"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/query"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/transaction"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type fakePoolProbe struct{}

func (f *fakePoolProbe) Ping(ctx context.Context) error { return nil }
func (f *fakePoolProbe) AcquiredConnections() int32     { return 0 }

type fakeLifecycleProbe struct{}

func (f *fakeLifecycleProbe) IsLive() bool  { return true }
func (f *fakeLifecycleProbe) IsReady() bool { return true }

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			RequestTimeout:  2 * time.Second,
			ShutdownTimeout: 2 * time.Second,
			MaxBodyBytes:    1024 * 1024,
		},
		Logging: config.LoggingConfig{Level: "info"},
		Auth: config.AuthConfig{
			EnableAPIKey: true,
			EnableJWT:    false,
			APIKeys: []config.APIKeyConfig{
				{
					Key:         "integration-test-api-key",
					Name:        "integration",
					Permissions: []string{"transaction:read", "balance:read"},
				},
			},
		},
		Tracing:     config.TracingConfig{Environment: "test"},
		Database:    config.DatabaseConfig{Host: "localhost", Database: "ledger", MaxConnections: 5},
		Idempotency: config.IdempotencyConfig{RetentionPeriod: time.Hour},
		Performance: config.PerformanceConfig{MaxBatchSize: 100},
		Currencies:  config.CurrenciesConfig{Supported: []string{"USD"}},
		NATS:        config.NATSConfig{Enabled: false},
	}
}

func testRouterMetrics() *metrics.Metrics {
	return &metrics.Metrics{
		HTTPRequestTotal:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_http_requests_total", Help: "h"}, []string{"method", "endpoint", "status"}),
		HTTPRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_http_request_duration_seconds", Help: "h"}, []string{"method", "endpoint"}),
		TransactionTotal:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_router_transaction_total", Help: "h"}, []string{"state", "status"}),
		TransactionErrors:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_router_transaction_errors_total", Help: "h"}, []string{"error_type"}),
		BatchTransactionTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_router_batch_transaction_total", Help: "h",
		}),
		BatchTransactionSize: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_router_batch_transaction_size", Help: "h"}),
		BalanceQueryTotal:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_router_balance_query_total", Help: "h"}, []string{"query_type", "status"}),
		BalanceQueryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_router_balance_query_duration", Help: "h"}),
		TransactionDuration:  prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_router_transaction_duration", Help: "h"}, []string{"operation"}),
		ActiveTransactions:   prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_router_active_transactions", Help: "h"}),
		DBConnectionsActive:  prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_router_db_connections_active", Help: "h"}),
		DBConnectionsIdle:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_router_db_connections_idle", Help: "h"}),
		DBConnectionsMax:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "test_router_db_connections_max", Help: "h"}),
		DBQueryDuration:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_router_db_query_duration", Help: "h"}, []string{"query_type"}),
		EventEmissionTotal:   prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_router_event_emission_total", Help: "h"}, []string{"event_type", "status"}),
		EventEmissionErrors:  prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_router_event_emission_errors", Help: "h"}, []string{"event_type"}),
	}
}

func buildTestRouter(t *testing.T) *Router {
	t.Helper()

	return NewRouter(
		&fakePoolProbe{},
		(transaction.TransactionProcessor)(nil),
		nil,
		(balance.BalanceCalculator)(nil),
		(query.QueryService)(nil),
		testConfig(),
		testRouterMetrics(),
		zap.NewNop(),
		&fakeLifecycleProbe{},
	)
}

func TestRouter_HealthEndpoint_NoAuthRequired(t *testing.T) {
	router := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRouter_ProtectedEndpoint_RequiresAuth(t *testing.T) {
	router := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/transactions", nil)
	rec := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRouter_SystemInfo_NoAuthRequired(t *testing.T) {
	router := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/system/info", nil)
	rec := httptest.NewRecorder()

	router.GetEngine().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

var _ handlers.Probe = (*fakeLifecycleProbe)(nil)
