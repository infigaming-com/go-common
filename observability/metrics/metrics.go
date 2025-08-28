package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// MetricExporter provides a generic interface for sending metrics
type MetricExporter struct {
	meterProvider    *sdkmetric.MeterProvider
	meter            metric.Meter
	resource         *resource.Resource
	serviceName      string
	serviceNamespace string
	serviceVersion   string
	otlpEndpoint     string
	otlpGRPCEndpoint string
	environment      string
}

// Option is a function that configures a MetricExporter
type Option func(*MetricExporter)

// WithServiceName sets the service name
func WithServiceName(name string) Option {
	return func(mc *MetricExporter) {
		mc.serviceName = name
	}
}

// WithServiceNamespace sets the service namespace
func WithServiceNamespace(namespace string) Option {
	return func(mc *MetricExporter) {
		mc.serviceNamespace = namespace
	}
}

// WithServiceVersion sets the service version
func WithServiceVersion(version string) Option {
	return func(mc *MetricExporter) {
		mc.serviceVersion = version
	}
}

// WithOTLPEndpoint sets the OTLP HTTP endpoint
func WithOTLPEndpoint(endpoint string) Option {
	return func(mc *MetricExporter) {
		mc.otlpEndpoint = endpoint
	}
}

// WithOTLPGRPCEndpoint sets the OTLP gRPC endpoint
func WithOTLPGRPCEndpoint(endpoint string) Option {
	return func(mc *MetricExporter) {
		mc.otlpGRPCEndpoint = endpoint
	}
}

// WithEnvironment sets the deployment environment
func WithEnvironment(env string) Option {
	return func(mc *MetricExporter) {
		mc.environment = env
	}
}

func defaultConfig() *MetricExporter {
	return &MetricExporter{
		serviceName:      "unknown-service",
		serviceNamespace: "default",
		serviceVersion:   "1.0.0",
		otlpEndpoint:     "localhost:4318",
		otlpGRPCEndpoint: "",
		environment:      "development",
	}
}

// NewMetricExporter creates a new metric exporter instance
func NewMetricExporter(opts ...Option) (*MetricExporter, func(), error) {
	// Create default client
	mc := defaultConfig()

	// Apply options
	for _, opt := range opts {
		opt(mc)
	}

	// Validate required fields
	if mc.otlpGRPCEndpoint != "" && mc.otlpEndpoint == "" {
		// gRPC endpoint is configured, HTTP endpoint is not required
	} else if mc.otlpEndpoint == "" {
		return nil, nil, fmt.Errorf("OTLP HTTP endpoint is required when gRPC endpoint is not configured")
	}

	// Create resource with service information
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(mc.serviceName),
			semconv.ServiceNamespace(mc.serviceNamespace),
			semconv.ServiceVersion(mc.serviceVersion),
			semconv.DeploymentEnvironment(mc.environment),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create OTLP exporter (HTTP or gRPC)
	var exporter sdkmetric.Exporter
	if mc.otlpGRPCEndpoint != "" {
		// Use gRPC if gRPC endpoint is configured
		exporter, err = otlpmetricgrpc.New(context.Background(),
			otlpmetricgrpc.WithEndpoint(mc.otlpGRPCEndpoint),
			otlpmetricgrpc.WithInsecure(), // Use TLS in production
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
		}
	} else {
		// Use HTTP if gRPC endpoint is not configured
		exporter, err = otlpmetrichttp.New(context.Background(),
			otlpmetrichttp.WithEndpoint(mc.otlpEndpoint),
			otlpmetrichttp.WithInsecure(), // Use TLS in production
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
		}
	}

	// Create meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter,
			sdkmetric.WithInterval(10*time.Second),
		)),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create meter
	meter := meterProvider.Meter(mc.serviceName)

	mc.meterProvider = meterProvider
	mc.meter = meter
	mc.resource = res

	return mc, func() {
		mc.meterProvider.Shutdown(context.Background())
	}, nil
}

// Close gracefully shuts down the metric exporter
func (mc *MetricExporter) Close(ctx context.Context) error {
	return mc.meterProvider.Shutdown(ctx)
}

// RecordCounter records a counter metric
func (mc *MetricExporter) RecordCounter(ctx context.Context, name, description, unit string, value int64, attributes map[string]string) error {
	counter, err := mc.meter.Int64Counter(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return fmt.Errorf("failed to create counter: %w", err)
	}

	// Convert attributes to key-value pairs
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	counter.Add(ctx, value, metric.WithAttributes(attrs...))
	return nil
}

// RecordGauge records a gauge metric
func (mc *MetricExporter) RecordGauge(ctx context.Context, name, description, unit string, value float64, attributes map[string]string) error {
	gauge, err := mc.meter.Float64ObservableGauge(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return fmt.Errorf("failed to create gauge: %w", err)
	}

	// Convert attributes to key-value pairs
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	// Register callback for gauge
	_, err = mc.meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveFloat64(gauge, value, metric.WithAttributes(attrs...))
		return nil
	}, gauge)
	if err != nil {
		return fmt.Errorf("failed to register gauge callback: %w", err)
	}

	return nil
}

// RecordHistogram records a histogram metric
func (mc *MetricExporter) RecordHistogram(ctx context.Context, name, description, unit string, value float64, attributes map[string]string) error {
	histogram, err := mc.meter.Float64Histogram(name,
		metric.WithDescription(description),
		metric.WithUnit(unit),
	)
	if err != nil {
		return fmt.Errorf("failed to create histogram: %w", err)
	}

	// Convert attributes to key-value pairs
	attrs := make([]attribute.KeyValue, 0, len(attributes))
	for k, v := range attributes {
		attrs = append(attrs, attribute.String(k, v))
	}

	histogram.Record(ctx, value, metric.WithAttributes(attrs...))
	return nil
}
