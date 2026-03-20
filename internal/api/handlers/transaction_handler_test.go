package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Eomaxl/double-entry-ledger-engine/internal/domain"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/infrastructure/metrics"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/query"
	"github.com/Eomaxl/double-entry-ledger-engine/internal/service/transaction"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type fakeTransactionProcessor struct {
	postFn func(ctx context.Context, req transaction.PostTransactionRequest) (*domain.Transaction, error)
}

func (f *fakeTransactionProcessor) PostTransaction(ctx context.Context, req transaction.PostTransactionRequest) (*domain.Transaction, error) {
	return f.postFn(ctx, req)
}
func (f *fakeTransactionProcessor) PostBatch(ctx context.Context, reqs []transaction.PostTransactionRequest) ([]domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionProcessor) SettlePendingTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionProcessor) CancelPendingTransaction(ctx context.Context, transactionID string) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeTransactionProcessor) ReverseTransaction(ctx context.Context, originalTransactionID string, req transaction.ReversalRequest) (*domain.Transaction, error) {
	return nil, nil
}

type fakeQueryService struct{}

func (f *fakeQueryService) GetTransaction(ctx context.Context, txnID string) (*domain.Transaction, error) {
	return nil, nil
}
func (f *fakeQueryService) ListTransactions(ctx context.Context, filter query.TransactionFilter) (*query.TransactionPage, error) {
	return nil, nil
}
func (f *fakeQueryService) GetAccountStatement(ctx context.Context, accountID string, filter query.StatementFilter) (*query.Statement, error) {
	return nil, nil
}

func testMetrics() *metrics.Metrics {
	return &metrics.Metrics{
		TransactionTotal: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_transaction_total", Help: "h"}, []string{"state", "status"}),
		TransactionErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_transaction_errors_total", Help: "h",
		}, []string{"error_type"}),
		BatchTransactionTotal: prometheus.NewCounter(prometheus.CounterOpts{Name: "test_batch_total", Help: "h"}),
		BatchTransactionSize:  prometheus.NewHistogram(prometheus.HistogramOpts{Name: "test_batch_size", Help: "h"}),
	}
}

func TestTransactionHandler_PostTransaction_Success(t *testing.T) {
	ginSetup(t)

	processor := &fakeTransactionProcessor{
		postFn: func(ctx context.Context, req transaction.PostTransactionRequest) (*domain.Transaction, error) {
			return &domain.Transaction{
				TransactionID: "tx-1",
				State:         domain.TransactionStateSettled,
				Entries:       []domain.Entry{},
				PostedAt:      time.Now(),
			}, nil
		},
	}

	h := NewTransactionHandler(processor, &fakeQueryService{}, testMetrics(), zap.NewNop())
	r := httptest.NewRecorder()
	engine := testEngine(func(c *ginContext) { h.PostTransaction(c.Context) })

	body := map[string]interface{}{
		"state": "settled",
		"entries": []map[string]interface{}{
			{"account_id": "550e8400-e29b-41d4-a716-446655440000", "currency_code": "USD", "amount": "10.00", "entry_type": "debit"},
			{"account_id": "550e8400-e29b-41d4-a716-446655440001", "currency_code": "USD", "amount": "10.00", "entry_type": "credit"},
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/transactions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(r, req)

	if r.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", r.Code, r.Body.String())
	}
}

func TestTransactionHandler_PostTransaction_InvalidBody(t *testing.T) {
	ginSetup(t)

	h := NewTransactionHandler(&fakeTransactionProcessor{}, &fakeQueryService{}, testMetrics(), zap.NewNop())
	r := httptest.NewRecorder()
	engine := testEngine(func(c *ginContext) { h.PostTransaction(c.Context) })

	req := httptest.NewRequest(http.MethodPost, "/v1/transactions", bytes.NewBufferString("{bad-json"))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(r, req)

	if r.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r.Code)
	}
}

// Lightweight wrappers keep test setup concise.
type ginContext struct{ Context *gin.Context }

func ginSetup(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

func testEngine(handler func(*ginContext)) *gin.Engine {
	e := gin.New()
	e.POST("/v1/transactions", func(c *gin.Context) { handler(&ginContext{Context: c}) })
	return e
}
