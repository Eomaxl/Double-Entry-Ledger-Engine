package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

type TracingConfig struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
	Environment string
}

func InitTracing(cfg TracingConfig) (func(context.Context) error, error) {
	if !cfg.Enabled {
		// Return no-op shutdown function
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion("1.0.0"),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(), // Use HTTPS in production
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Configure sampling in production
	)

	// Set global trace provider
	otel.SetTracerProvider(tp)

	// Set global propagator for trace context propagation
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return shutdown function
	return tp.Shutdown, nil
}

// GetTracer returns a tracer for the ledger service
func GetTracer() trace.Tracer {
	return otel.Tracer("github.com/Eomaxl/double-entry-ledger-engine")
}

// StartSpan starts a new span with common attributes
func StartSpan(ctx context.Context, operationName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	tracer := GetTracer()
	return tracer.Start(ctx, operationName, trace.WithAttributes(attrs...))
}

// StartTransactionSpan starts a span for transaction operations
func StartTransactionSpan(ctx context.Context, operation string, transactionID string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("ledger.operation", operation),
		attribute.String("ledger.transaction_id", transactionID),
	}
	baseAttrs = append(baseAttrs, attrs...)

	return StartSpan(ctx, fmt.Sprintf("transaction.%s", operation), baseAttrs...)
}

// StartValidationSpan starts a span for validation operations
func StartValidationSpan(ctx context.Context, validationType string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("ledger.validation_type", validationType),
	}
	baseAttrs = append(baseAttrs, attrs...)

	return StartSpan(ctx, fmt.Sprintf("validation.%s", validationType), baseAttrs...)
}

// StartDatabaseSpan starts a span for database operations
func StartDatabaseSpan(ctx context.Context, operation string, table string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("db.operation", operation),
		attribute.String("db.table", table),
		attribute.String("db.system", "postgresql"),
	}
	baseAttrs = append(baseAttrs, attrs...)

	return StartSpan(ctx, fmt.Sprintf("db.%s", operation), baseAttrs...)
}

// StartEventSpan starts a span for event emission operations
func StartEventSpan(ctx context.Context, eventType string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("ledger.event_type", eventType),
		attribute.String("messaging.system", "nats"),
	}
	baseAttrs = append(baseAttrs, attrs...)

	return StartSpan(ctx, fmt.Sprintf("event.%s", eventType), baseAttrs...)
}

// AddAccountAttributes adds account-related attributes to a span
func AddAccountAttributes(span trace.Span, accountID string, currency string) {
	span.SetAttributes(
		attribute.String("ledger.account_id", accountID),
		attribute.String("ledger.currency", currency),
	)
}

// AddTransactionAttributes adds transaction-related attributes to a span
func AddTransactionAttributes(span trace.Span, transactionID string, entryCount int, totalAmount string) {
	span.SetAttributes(
		attribute.String("ledger.transaction_id", transactionID),
		attribute.Int("ledger.entry_count", entryCount),
		attribute.String("ledger.total_amount", totalAmount),
	)
}

// AddErrorAttributes adds error information to a span
func AddErrorAttributes(span trace.Span, err error) {
	if err != nil {
		span.SetAttributes(
			attribute.Bool("error", true),
			attribute.String("error.message", err.Error()),
		)
		span.RecordError(err)
	}
}

// PropagateTraceContext propagates trace context to NATS message headers
func PropagateTraceContext(ctx context.Context, headers map[string]string) {
	if headers == nil {
		headers = make(map[string]string)
	}

	// Inject trace context into headers
	otel.GetTextMapPropagator().Inject(ctx, &HeaderCarrier{headers: headers})
}

// ExtractTraceContext extracts trace context from NATS message headers
func ExtractTraceContext(ctx context.Context, headers map[string]string) context.Context {
	if headers == nil {
		return ctx
	}

	// Extract trace context from headers
	return otel.GetTextMapPropagator().Extract(ctx, &HeaderCarrier{headers: headers})
}

// HeaderCarrier implements TextMapCarrier for NATS headers
type HeaderCarrier struct {
	headers map[string]string
}

func (hc *HeaderCarrier) Get(key string) string {
	return hc.headers[key]
}

func (hc *HeaderCarrier) Set(key, value string) {
	hc.headers[key] = value
}

func (hc *HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(hc.headers))
	for k := range hc.headers {
		keys = append(keys, k)
	}
	return keys
}
