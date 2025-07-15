package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestNewResourceMetrics(t *testing.T) {
	tests := []struct {
		name    string
		logger  logr.Logger
		wantErr bool
	}{
		{
			name:    "valid logger",
			logger:  logr.Discard(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewResourceMetrics(tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResourceMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Errorf("NewResourceMetrics() got = nil, want non-nil")
			}
		})
	}
}

func TestResourceMetrics_RecordQueueDepth(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		queueName string
		depth     int64
	}{
		{
			name:      "webhook queue",
			queueName: "webhook",
			depth:     10,
		},
		{
			name:      "controller queue",
			queueName: "controller",
			depth:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm.RecordQueueDepth(ctx, tt.queueName, tt.depth)
			// Test passes if no panic occurs
		})
	}
}

func TestResourceMetrics_RecordQueueProcessingTime(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name           string
		queueName      string
		processingTime time.Duration
	}{
		{
			name:           "webhook processing",
			queueName:      "webhook",
			processingTime: 50 * time.Millisecond,
		},
		{
			name:           "policy processing",
			queueName:      "policy",
			processingTime: 25 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm.RecordQueueProcessingTime(ctx, tt.queueName, tt.processingTime)
			// Test passes if no panic occurs
		})
	}
}

// BenchmarkResourceMetrics tests the performance impact of resource metrics
func BenchmarkResourceMetrics_RecordQueueDepth(b *testing.B) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordQueueDepth(ctx, "test-queue", int64(i%100))
	}
}

func BenchmarkResourceMetrics_RecordQueueProcessingTime(b *testing.B) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordQueueProcessingTime(ctx, "test-queue", time.Duration(i%1000)*time.Microsecond)
	}
}

// TestResourceMetrics_Creation tests that ResourceMetrics can be created successfully
// and all internal instruments are initialized without errors
func TestResourceMetrics_Creation(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	// Test that the struct was created with expected fields
	if rm == nil {
		t.Error("Expected ResourceMetrics to be non-nil")
	}

	// Test that all queue methods work without panicking
	ctx := context.Background()
	rm.RecordQueueDepth(ctx, "test", 5)
	rm.RecordQueueProcessingTime(ctx, "test", 100*time.Millisecond)
}
