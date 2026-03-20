package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/balance"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type fakeBalanceCalculator struct {
	getCurrentErr error
}

func (f *fakeBalanceCalculator) GetCurrentBalance(ctx context.Context, accountID uuid.UUID, currency string) (*domain.Balance, error) {
	if f.getCurrentErr != nil {
		return nil, f.getCurrentErr
	}
	return &domain.Balance{AccountID: accountID.String(), Currency: currency}, nil
}
func (f *fakeBalanceCalculator) GetCurrentBalanceInTx(ctx context.Context, tx pgx.Tx, accountID uuid.UUID, currency string) (*domain.Balance, error) {
	return nil, nil
}
func (f *fakeBalanceCalculator) GetHistoricalBalance(ctx context.Context, accountID uuid.UUID, currency string, asOf time.Time) (*domain.Balance, error) {
	return &domain.Balance{AccountID: accountID.String(), Currency: currency}, nil
}
func (f *fakeBalanceCalculator) GetMultiCurrencyBalance(ctx context.Context, accountID uuid.UUID) (map[string]*domain.Balance, error) {
	return map[string]*domain.Balance{}, nil
}

func balanceMetrics() *metrics.Metrics {
	return &metrics.Metrics{
		BalanceQueryTotal:    prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_balance_total", Help: "h"}, []string{"query_type", "status"}),
		BalanceQueryDuration: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_balance_duration", Help: "h"}),
	}
}

func TestBalanceHandler_GetCurrentBalance_InvalidCurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBalanceHandler(&fakeBalanceCalculator{}, balanceMetrics(), zap.NewNop())
	r := gin.New()
	r.GET("/v1/accounts/:id/balances/:currency", h.GetCurrentBalance)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/550e8400-e29b-41d4-a716-446655440000/balances/USDD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

func TestBalanceHandler_GetHistoricalBalance_MissingAsOf(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBalanceHandler(&fakeBalanceCalculator{}, balanceMetrics(), zap.NewNop())
	r := gin.New()
	r.GET("/v1/accounts/:id/balances/:currency/history", h.GetHistoricalBalance)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/550e8400-e29b-41d4-a716-446655440000/balances/USD/history", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", rec.Code)
	}
}

func TestBalanceHandler_GetCurrentBalance_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewBalanceHandler(&fakeBalanceCalculator{getCurrentErr: errors.New("database connection failed")}, balanceMetrics(), zap.NewNop())
	r := gin.New()
	r.GET("/v1/accounts/:id/balances/:currency", h.GetCurrentBalance)

	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/550e8400-e29b-41d4-a716-446655440000/balances/USD", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 got %d body=%s", rec.Code, rec.Body.String())
	}
}

var _ balance.BalanceCalculator = (*fakeBalanceCalculator)(nil)
