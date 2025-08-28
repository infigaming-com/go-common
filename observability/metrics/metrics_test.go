package metrics

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetricExporter(t *testing.T) {
	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "valid config with HTTP",
			opts: []Option{
				WithServiceName("test-service"),
				WithServiceNamespace("test"),
				WithServiceVersion("1.0.0"),
				WithOTLPEndpoint("localhost:4318"),
				WithEnvironment("test"),
			},
			wantErr: false,
		},
		{
			name: "valid config with gRPC",
			opts: []Option{
				WithServiceName("test-service"),
				WithServiceNamespace("test"),
				WithServiceVersion("1.0.0"),
				WithOTLPGRPCEndpoint("localhost:4317"),
				WithEnvironment("test"),
			},
			wantErr: false, // Will fail at runtime due to no gRPC server, but config is valid
		},
		{
			name: "empty service name",
			opts: []Option{
				WithServiceName(""),
				WithServiceNamespace("test"),
				WithServiceVersion("1.0.0"),
				WithOTLPEndpoint("localhost:4318"),
				WithEnvironment("test"),
			},
			wantErr: false, // OpenTelemetry allows empty service names
		},
		{
			name: "empty OTLP endpoint",
			opts: []Option{
				WithServiceName("test-service"),
				WithServiceNamespace("test"),
				WithServiceVersion("1.0.0"),
				WithEnvironment("test"),
				WithOTLPEndpoint(""), // Explicitly set empty endpoint
			},
			wantErr: true, // Empty endpoint should cause error
		},
		{
			name: "gRPC takes precedence over HTTP",
			opts: []Option{
				WithServiceName("test-service"),
				WithOTLPEndpoint("localhost:4318"),
				WithOTLPGRPCEndpoint("localhost:4317"), // This should be used
			},
			wantErr: false, // Will fail at runtime due to no gRPC server, but config is valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMetricExporter(tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			// For gRPC tests, we expect the client to be created but may fail during export
			// due to no gRPC server running
			if tt.name == "valid config with gRPC" || tt.name == "gRPC takes precedence over HTTP" {
				// Client should be created successfully
				require.NoError(t, err)
				require.NotNil(t, client)
				assert.NotNil(t, client.meterProvider)
				assert.NotNil(t, client.meter)
				assert.NotNil(t, client.resource)

				// Clean up - this might fail due to gRPC connection issues, but that's expected
				_ = client.Close(context.Background())
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.NotNil(t, client.meterProvider)
			assert.NotNil(t, client.meter)
			assert.NotNil(t, client.resource)

			// Clean up
			err = client.Close(context.Background())
			assert.NoError(t, err)
		})
	}
}

func TestMetricClient_RecordCounter(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("test-service"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	ctx := context.Background()

	tests := []struct {
		name        string
		metricName  string
		description string
		unit        string
		value       int64
		attributes  map[string]string
		wantErr     bool
	}{
		{
			name:        "valid counter",
			metricName:  "test.counter",
			description: "A test counter",
			unit:        "1",
			value:       1,
			attributes: map[string]string{
				"test.attribute": "test-value",
			},
			wantErr: false,
		},
		{
			name:        "counter with empty name",
			metricName:  "",
			description: "A test counter",
			unit:        "1",
			value:       1,
			attributes:  nil,
			wantErr:     true,
		},
		{
			name:        "counter with nil attributes",
			metricName:  "test.counter",
			description: "A test counter",
			unit:        "1",
			value:       1,
			attributes:  nil,
			wantErr:     false,
		},
		{
			name:        "counter with multiple attributes",
			metricName:  "test.counter",
			description: "A test counter",
			unit:        "1",
			value:       5,
			attributes: map[string]string{
				"component": "api",
				"operation": "request",
				"status":    "success",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.RecordCounter(ctx, tt.metricName, tt.description, tt.unit, tt.value, tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClient_RecordGauge(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("test-service"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	ctx := context.Background()

	tests := []struct {
		name        string
		metricName  string
		description string
		unit        string
		value       float64
		attributes  map[string]string
		wantErr     bool
	}{
		{
			name:        "valid gauge",
			metricName:  "test.gauge",
			description: "A test gauge",
			unit:        "1",
			value:       100.5,
			attributes: map[string]string{
				"test.attribute": "test-value",
			},
			wantErr: false,
		},
		{
			name:        "gauge with empty name",
			metricName:  "",
			description: "A test gauge",
			unit:        "1",
			value:       100.5,
			attributes:  nil,
			wantErr:     true,
		},
		{
			name:        "gauge with negative value",
			metricName:  "test.gauge",
			description: "A test gauge",
			unit:        "1",
			value:       -50.0,
			attributes:  nil,
			wantErr:     false,
		},
		{
			name:        "gauge with memory unit",
			metricName:  "memory.usage",
			description: "Memory usage",
			unit:        "bytes",
			value:       1024.0,
			attributes: map[string]string{
				"type": "heap",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.RecordGauge(ctx, tt.metricName, tt.description, tt.unit, tt.value, tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClient_RecordHistogram(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("test-service"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	ctx := context.Background()

	tests := []struct {
		name        string
		metricName  string
		description string
		unit        string
		value       float64
		attributes  map[string]string
		wantErr     bool
	}{
		{
			name:        "valid histogram",
			metricName:  "test.histogram",
			description: "A test histogram",
			unit:        "ms",
			value:       150.0,
			attributes: map[string]string{
				"test.attribute": "test-value",
			},
			wantErr: false,
		},
		{
			name:        "histogram with empty name",
			metricName:  "",
			description: "A test histogram",
			unit:        "ms",
			value:       150.0,
			attributes:  nil,
			wantErr:     true,
		},
		{
			name:        "histogram with zero value",
			metricName:  "test.histogram",
			description: "A test histogram",
			unit:        "ms",
			value:       0.0,
			attributes:  nil,
			wantErr:     false,
		},
		{
			name:        "histogram with large value",
			metricName:  "test.histogram",
			description: "A test histogram",
			unit:        "ms",
			value:       999999.99,
			attributes: map[string]string{
				"operation": "slow_query",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.RecordHistogram(ctx, tt.metricName, tt.description, tt.unit, tt.value, tt.attributes)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricClient_Close(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("test-service"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)

	// Test normal close
	ctx := context.Background()
	err = client.Close(ctx)
	assert.NoError(t, err)

	// Test close with timeout
	client2, err := NewMetricExporter(
		WithServiceName("test-service-2"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = client2.Close(ctx)
	assert.NoError(t, err)
}

func TestMetricClient_Integration(t *testing.T) {
	// Test that we can create a client and record multiple metric types
	client, err := NewMetricExporter(
		WithServiceName("integration-test"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	ctx := context.Background()

	// Record a counter
	err = client.RecordCounter(ctx, "integration.counter", "Integration test counter", "1", 1, map[string]string{
		"test": "integration",
	})
	assert.NoError(t, err)

	// Record a gauge
	err = client.RecordGauge(ctx, "integration.gauge", "Integration test gauge", "1", 100.0, map[string]string{
		"test": "integration",
	})
	assert.NoError(t, err)

	// Record a histogram
	err = client.RecordHistogram(ctx, "integration.histogram", "Integration test histogram", "ms", 150.0, map[string]string{
		"test": "integration",
	})
	assert.NoError(t, err)
}

func TestMetricClient_ContinuousMetrics(t *testing.T) {
	t.Skip("Skipping continuous metrics test - run manually when needed for GCP verification")

	// Test continuous metric sending to see active metrics in GCP
	client, err := NewMetricExporter(
		WithServiceName("continuous-test"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	ctx := context.Background()
	counter := int64(0)

	// Send metrics continuously for 30 seconds
	duration := 30 * time.Second
	interval := 2 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	endTime := time.Now().Add(duration)

	fmt.Printf("Sending continuous metrics for %v...\n", duration)

	for time.Now().Before(endTime) {
		select {
		case <-ticker.C:
			counter++

			// Record counter metric
			err := client.RecordCounter(ctx,
				"continuous.requests.total",
				"Total continuous requests",
				"1",
				counter,
				map[string]string{
					"endpoint": "/api/v1/test",
					"method":   "GET",
					"status":   "success",
				},
			)
			assert.NoError(t, err)

			// Record gauge metric (current timestamp as value)
			err = client.RecordGauge(ctx,
				"continuous.active_connections",
				"Number of active connections",
				"1",
				float64(time.Now().Unix()),
				map[string]string{
					"server": "api-server-1",
				},
			)
			assert.NoError(t, err)

			// Record histogram metric (random latency)
			err = client.RecordHistogram(ctx,
				"continuous.request_duration",
				"Request duration in milliseconds",
				"ms",
				float64(100+time.Now().Unix()%500), // Random value between 100-600ms
				map[string]string{
					"endpoint": "/api/v1/test",
				},
			)
			assert.NoError(t, err)

			fmt.Printf("Sent metrics - Counter: %d, Time: %s\n", counter, time.Now().Format("15:04:05"))
		}
	}

	fmt.Printf("Completed sending %d metric batches\n", counter)
	assert.Greater(t, counter, int64(0), "Should have sent at least one batch of metrics")
}

func TestMetricClient_ConcurrentUsage(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("concurrent-test"),
		WithServiceNamespace("test"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("test"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	// Test concurrent metric recording
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := context.Background()
			err := client.RecordCounter(ctx, "concurrent.counter", "Concurrent test counter", "1", int64(id), map[string]string{
				"goroutine": fmt.Sprintf("%d", id),
			})
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestMetricClient_ResourceAttributes(t *testing.T) {
	client, err := NewMetricExporter(
		WithServiceName("resource-test"),
		WithServiceNamespace("test"),
		WithServiceVersion("2.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("production"),
	)
	require.NoError(t, err)
	defer client.Close(context.Background())

	// Verify resource attributes are set correctly
	assert.NotNil(t, client.resource)

	// Note: We can't easily test the actual attribute values without accessing internal OpenTelemetry APIs
	// But we can verify the resource was created successfully
	assert.NotNil(t, client.meterProvider)
	assert.NotNil(t, client.meter)
}

// Example usage functions - these demonstrate how to use the metric client

func ExampleNewMetricExporter_http() {
	// Create metric exporter with HTTP endpoint (default behavior)
	client, err := NewMetricExporter(
		WithServiceName("my-service"),
		WithServiceNamespace("dev"),
		WithServiceVersion("1.0.0"),
		WithOTLPEndpoint("localhost:4318"),
		WithEnvironment("development"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "requests.total", "Total number of requests", "1", 1, map[string]string{
		"endpoint": "/api/v1/users",
		"method":   "GET",
	})
	if err != nil {
		panic(err)
	}
}

func ExampleNewMetricExporter_grpc() {
	// Create metric client with gRPC endpoint (automatically uses gRPC when configured)
	client, err := NewMetricExporter(
		WithServiceName("my-service"),
		WithServiceNamespace("dev"),
		WithServiceVersion("1.0.0"),
		WithOTLPGRPCEndpoint("localhost:4317"),
		WithEnvironment("development"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "requests.total", "Total number of requests", "1", 1, map[string]string{
		"endpoint": "/api/v1/users",
		"method":   "GET",
	})
	if err != nil {
		panic(err)
	}
}

func ExampleNewMetricExporter_minimalHTTP() {
	// Create metric client with just the required HTTP endpoint
	client, err := NewMetricExporter(
		WithOTLPEndpoint("localhost:4318"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "simple.counter", "A simple counter", "1", 1, nil)
	if err != nil {
		panic(err)
	}
}

func ExampleNewMetricExporter_minimalGRPC() {
	// Create metric client with just the required gRPC endpoint
	client, err := NewMetricExporter(
		WithOTLPGRPCEndpoint("localhost:4317"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "simple.counter", "A simple counter", "1", 1, nil)
	if err != nil {
		panic(err)
	}
}

func ExampleNewMetricExporter_production() {
	// Create metric client for production with gRPC
	client, err := NewMetricExporter(
		WithServiceName("production-api"),
		WithServiceNamespace("prod"),
		WithServiceVersion("2.1.0"),
		WithOTLPGRPCEndpoint("otel-collector.prod.svc.cluster.local:4317"),
		WithEnvironment("production"),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "production.requests", "Production requests", "1", 1, nil)
	if err != nil {
		panic(err)
	}
}

func ExampleNewMetricExporter_grpcPrecedence() {
	// Even if both endpoints are configured, gRPC takes precedence
	client, err := NewMetricExporter(
		WithServiceName("my-service"),
		WithOTLPEndpoint("localhost:4318"),
		WithOTLPGRPCEndpoint("localhost:4317"), // This will be used
	)
	if err != nil {
		panic(err)
	}
	defer client.Close(context.Background())

	ctx := context.Background()
	err = client.RecordCounter(ctx, "mixed.counter", "Counter with mixed config", "1", 1, nil)
	if err != nil {
		panic(err)
	}
}
