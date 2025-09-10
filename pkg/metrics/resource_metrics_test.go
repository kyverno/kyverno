package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func TestNewResourceMetrics(t *testing.T) {
	tests := []struct {
		name           string
		setupProvider  func() *sdkmetric.MeterProvider
		logger         logr.Logger
		wantErr        bool
		expectNonNil   bool
		expectNoOpMode bool
	}{
		{
			name: "valid logger with meter provider",
			setupProvider: func() *sdkmetric.MeterProvider {
				reader := sdkmetric.NewManualReader()
				return sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			},
			logger:         logr.Discard(),
			wantErr:        false,
			expectNonNil:   true,
			expectNoOpMode: false,
		},
		{
			name: "valid logger without meter provider",
			setupProvider: func() *sdkmetric.MeterProvider {
				return nil
			},
			logger:         logr.Discard(),
			wantErr:        false,
			expectNonNil:   true,
			expectNoOpMode: true,
		},
		{
			name: "logger with no-op meter provider",
			setupProvider: func() *sdkmetric.MeterProvider {
				return sdkmetric.NewMeterProvider()
			},
			logger:         logr.Discard(),
			wantErr:        false,
			expectNonNil:   true,
			expectNoOpMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup meter provider
			provider := tt.setupProvider()
			if provider != nil {
				otel.SetMeterProvider(provider)
				defer func() {
					if err := provider.Shutdown(context.Background()); err != nil {
						t.Logf("Failed to shutdown meter provider: %v", err)
					}
				}()
			} else {
				otel.SetMeterProvider(nil)
			}

			got, err := NewResourceMetrics(tt.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewResourceMetrics() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.expectNonNil && got == nil {
				t.Errorf("NewResourceMetrics() got = nil, want non-nil")
			}
			if tt.expectNonNil && got == nil {
				t.Errorf("NewResourceMetrics() got = nil, want non-nil")
			}

			// Check if we're in no-op mode when expected
			if tt.expectNoOpMode && got != nil {
				if got.meter != nil {
					t.Errorf("Expected no-op mode but got meter")
				}
			}
		})
	}
}

func TestResourceMetrics_RecordCacheHit(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		cacheType string
	}{
		{
			name:      "policy cache hit",
			cacheType: "policy",
		},
		{
			name:      "image cache hit",
			cacheType: "image",
		},
		{
			name:      "empty cache type",
			cacheType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			rm.RecordCacheHit(ctx, tt.cacheType)
		})
	}
}

func TestResourceMetrics_RecordCacheMiss(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name      string
		cacheType string
	}{
		{
			name:      "policy cache miss",
			cacheType: "policy",
		},
		{
			name:      "validation cache miss",
			cacheType: "validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			rm.RecordCacheMiss(ctx, tt.cacheType)
		})
	}
}

func TestResourceMetrics_RecordProcessingTime(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name          string
		operationType string
		duration      time.Duration
	}{
		{
			name:          "policy validation",
			operationType: "validation",
			duration:      50 * time.Millisecond,
		},
		{
			name:          "policy mutation",
			operationType: "mutation",
			duration:      25 * time.Millisecond,
		},
		{
			name:          "zero duration",
			operationType: "generation",
			duration:      0,
		},
		{
			name:          "long duration",
			operationType: "image-verification",
			duration:      5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			rm.RecordProcessingTime(ctx, tt.operationType, tt.duration)
		})
	}
}

func TestResourceMetrics_RecordAdmissionLatency(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name         string
		duration     time.Duration
		resourceKind string
	}{
		{
			name:         "pod admission",
			duration:     100 * time.Millisecond,
			resourceKind: "Pod",
		},
		{
			name:         "deployment admission",
			duration:     200 * time.Millisecond,
			resourceKind: "Deployment",
		},
		{
			name:         "service admission",
			duration:     50 * time.Millisecond,
			resourceKind: "Service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			rm.RecordAdmissionLatency(ctx, tt.duration, tt.resourceKind)
		})
	}
}

func TestResourceMetrics_RecordGCCycle(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	// Should not panic
	rm.RecordGCCycle(ctx)
	rm.RecordGCCycle(ctx) // Call multiple times
}

func TestResourceMetrics_NoOpMode(t *testing.T) {
	// Test behavior when no meter provider is available
	otel.SetMeterProvider(nil)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("NewResourceMetrics() failed in no-op mode: %v", err)
	}

	if rm == nil {
		t.Fatal("Expected non-nil ResourceMetrics in no-op mode")
	}

	ctx := context.Background()

	// All these should work without panicking in no-op mode
	rm.RecordCacheHit(ctx, "test")
	rm.RecordCacheMiss(ctx, "test")
	rm.RecordProcessingTime(ctx, "test", time.Millisecond)
	rm.RecordAdmissionLatency(ctx, time.Millisecond, "Pod")
	rm.RecordGCCycle(ctx)
}

func TestResourceMetrics_CollectRuntimeMetrics(t *testing.T) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	_, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	// Force collection of runtime metrics
	var rm metricdata.ResourceMetrics
	err = reader.Collect(context.Background(), &rm)
	if err != nil {
		t.Errorf("Failed to collect metrics: %v", err)
	}

	// Verify that metrics were collected (basic smoke test)
	// The actual values are not deterministic, so we just check that collection works
}

// BenchmarkResourceMetrics tests the performance impact of resource metrics
func BenchmarkResourceMetrics_RecordCacheHit(b *testing.B) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			b.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordCacheHit(ctx, "test-cache")
	}
}

func BenchmarkResourceMetrics_RecordProcessingTime(b *testing.B) {
	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			b.Logf("Failed to shutdown meter provider: %v", err)
		}
	}()

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordProcessingTime(ctx, "validation", time.Duration(i%1000)*time.Microsecond)
	}
}

func BenchmarkResourceMetrics_NoOpMode(b *testing.B) {
	// Test performance in no-op mode
	otel.SetMeterProvider(nil)

	rm, err := NewResourceMetrics(logr.Discard())
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordCacheHit(ctx, "test-cache")
		rm.RecordProcessingTime(ctx, "validation", time.Microsecond)
	}
}
