package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	TransactionTotal      *prometheus.CounterVec
	TransactionDuration   *prometheus.HistogramVec
	TransactionErrors     *prometheus.CounterVec
	ActiveTransactions    prometheus.Gauge
	BatchTransactionTotal prometheus.Counter
	BatchTransactionSize  prometheus.Histogram

	BalanceQueryTotal    *prometheus.CounterVec
	BalanceQueryDuration prometheus.Histogram

	DBConnectionsActive prometheus.Gauge
	DBConnectionsIdle   prometheus.Gauge
	DBConnectionsMax    prometheus.Gauge
	DBQueryDuration     *prometheus.HistogramVec

	EventEmissionTotal  *prometheus.CounterVec
	EventEmissionErrors *prometheus.CounterVec

	HTTPRequestTotal    *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		TransactionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_transactions_total",
				Help: "Total number of transactions processed",
			},
			[]string{"state", "status"},
		),
		TransactionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ledger_transaction_duration_seconds",
				Help:    "Transaction processing duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		TransactionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_transaction_errors_total",
				Help: "Total number of transaction errors",
			},
			[]string{"error_type"},
		),
		ActiveTransactions: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "ledger_active_transactions",
				Help: "Number of currently active transactions",
			},
		),
		BatchTransactionTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "ledger_batch_transactions_total",
				Help: "Total number of batch transactions processed",
			},
		),
		BatchTransactionSize: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "ledger_batch_transaction_size",
				Help:    "Number of transactions in each batch",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			},
		),

		// Balance query metrics
		BalanceQueryTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_balance_queries_total",
				Help: "Total number of balance queries",
			},
			[]string{"query_type", "status"},
		),
		BalanceQueryDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "ledger_balance_query_duration_seconds",
				Help:    "Balance query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
			},
		),

		// Database metrics
		DBConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "ledger_db_connections_active",
				Help: "Number of active database connections",
			},
		),
		DBConnectionsIdle: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "ledger_db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		DBConnectionsMax: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "ledger_db_connections_max",
				Help: "Maximum number of database connections",
			},
		),
		DBQueryDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ledger_db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"query_type"},
		),

		// Event emission metrics
		EventEmissionTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_event_emissions_total",
				Help: "Total number of events emitted",
			},
			[]string{"event_type", "status"},
		),
		EventEmissionErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_event_emission_errors_total",
				Help: "Total number of event emission errors",
			},
			[]string{"event_type"},
		),

		// API metrics
		HTTPRequestTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ledger_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ledger_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
	}
}
