package metrics

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kyverno/kyverno/pkg/config"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/trace"
	corev1 "k8s.io/api/core/v1"
)

func Test_aggregationSelector(t *testing.T) {
	tests := []struct {
		name           string
		config         kconfig.MetricsConfiguration
		instrumentKind sdkmetric.InstrumentKind
		expected       sdkmetric.Aggregation
	}{
		{
			name:           "histogram instrument with default config",
			config:         kconfig.NewDefaultMetricsConfiguration(),
			instrumentKind: sdkmetric.InstrumentKindHistogram,
			expected: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
				NoMinMax:   true, // This is the critical fix - must be true to prevent memory leak
			},
		},
		{
			name:           "counter instrument uses default aggregation",
			config:         kconfig.NewDefaultMetricsConfiguration(),
			instrumentKind: sdkmetric.InstrumentKindCounter,
			expected:       sdkmetric.DefaultAggregationSelector(sdkmetric.InstrumentKindCounter),
		},
		{
			name:           "gauge instrument uses default aggregation",
			config:         kconfig.NewDefaultMetricsConfiguration(),
			instrumentKind: sdkmetric.InstrumentKindGauge,
			expected:       sdkmetric.DefaultAggregationSelector(sdkmetric.InstrumentKindGauge),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := aggregationSelector(tt.config)
			result := selector(tt.instrumentKind)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("aggregationSelector() = %v, want %v", result, tt.expected)
			}

			// Specifically test the NoMinMax fix for histograms
			if tt.instrumentKind == sdkmetric.InstrumentKindHistogram {
				if hist, ok := result.(sdkmetric.AggregationExplicitBucketHistogram); ok {
					if !hist.NoMinMax {
						t.Errorf("aggregationSelector() for histogram must have NoMinMax=true to prevent memory leak, got NoMinMax=false")
					}
				} else {
					t.Errorf("aggregationSelector() for histogram should return AggregationExplicitBucketHistogram, got %T", result)
				}
			}
		})
	}
}

func Test_aggregationSelector_memoryLeakPrevention(t *testing.T) {
	// This test specifically validates the memory leak fix
	config := kconfig.NewDefaultMetricsConfiguration()
	selector := aggregationSelector(config)

	// Test histogram aggregation
	histogramAgg := selector(sdkmetric.InstrumentKindHistogram)

	// Verify it's the correct type
	hist, ok := histogramAgg.(sdkmetric.AggregationExplicitBucketHistogram)
	if !ok {
		t.Fatalf("Expected AggregationExplicitBucketHistogram, got %T", histogramAgg)
	}

	// Critical test: NoMinMax must be true to prevent memory leak
	if !hist.NoMinMax {
		t.Errorf("MEMORY LEAK: NoMinMax must be true to prevent histogram min/max accumulation. Got NoMinMax=%v", hist.NoMinMax)
	}

	// Verify boundaries are set correctly
	expectedBoundaries := config.GetBucketBoundaries()
	if !reflect.DeepEqual(hist.Boundaries, expectedBoundaries) {
		t.Errorf("Bucket boundaries mismatch. Expected %v, got %v", expectedBoundaries, hist.Boundaries)
	}
}

func Test_aggregationSelector_withCustomBoundaries(t *testing.T) {
	// Create a mock config with custom boundaries
	config := &mockMetricsConfig{
		bucketBoundaries: []float64{0.1, 0.5, 1.0, 5.0},
	}

	selector := aggregationSelector(config)
	result := selector(sdkmetric.InstrumentKindHistogram)

	hist, ok := result.(sdkmetric.AggregationExplicitBucketHistogram)
	if !ok {
		t.Fatalf("Expected AggregationExplicitBucketHistogram, got %T", result)
	}

	// Verify custom boundaries
	expected := []float64{0.1, 0.5, 1.0, 5.0}
	if !reflect.DeepEqual(hist.Boundaries, expected) {
		t.Errorf("Expected custom boundaries %v, got %v", expected, hist.Boundaries)
	}

	// Critical: NoMinMax must still be true even with custom boundaries
	if !hist.NoMinMax {
		t.Errorf("NoMinMax must be true even with custom boundaries to prevent memory leak")
	}
}

// Mock config for testing
type mockMetricsConfig struct {
	bucketBoundaries []float64
}

func (m *mockMetricsConfig) GetBucketBoundaries() []float64 {
	return m.bucketBoundaries
}

func (m *mockMetricsConfig) GetExcludeNamespaces() []string {
	return []string{}
}

func (m *mockMetricsConfig) GetIncludeNamespaces() []string {
	return []string{}
}

func (m *mockMetricsConfig) GetMetricsRefreshInterval() time.Duration {
	return time.Second * 30
}

func (m *mockMetricsConfig) CheckNamespace(string) bool {
	return true
}

func (m *mockMetricsConfig) BuildMeterProviderViews() []sdkmetric.View {
	return []sdkmetric.View{}
}

func (m *mockMetricsConfig) Load(*corev1.ConfigMap) {}

func (m *mockMetricsConfig) OnChanged(func()) {}

func sampledContext() context.Context {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01},
		SpanID:     trace.SpanID{0x01},
		TraceFlags: trace.FlagsSampled,
	})
	return trace.ContextWithSpanContext(context.Background(), sc)
}

func TestResolveExemplarFilter(t *testing.T) {
	t.Parallel()

	bgCtx := context.Background()
	sampledCtx := sampledContext()

	tests := []struct {
		name            string
		value           string
		bgExpected      bool
		sampledExpected bool
	}{
		{
			name:            "always-off returns AlwaysOffFilter behaviour",
			value:           "always-off",
			bgExpected:      false,
			sampledExpected: false,
		},
		{
			name:            "empty string defaults to TraceBasedFilter behaviour",
			value:           "",
			bgExpected:      false,
			sampledExpected: true,
		},
		{
			name:            "unknown value defaults to TraceBasedFilter behaviour",
			value:           "invalid",
			bgExpected:      false,
			sampledExpected: true,
		},
		{
			name:            "trace-based returns TraceBasedFilter behaviour",
			value:           "trace-based",
			bgExpected:      false,
			sampledExpected: true,
		},
		{
			name:            "always-on returns AlwaysOnFilter behaviour",
			value:           "always-on",
			bgExpected:      true,
			sampledExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			filter := resolveExemplarFilter(tt.value)
			assert.Equal(t, tt.bgExpected, filter(bgCtx), "background context")
			assert.Equal(t, tt.sampledExpected, filter(sampledCtx), "sampled context")
		})
	}
}

func TestResolveExemplarFilterBehaviourConsistency(t *testing.T) {
	t.Parallel()

	bgCtx := context.Background()
	sampledCtx := sampledContext()

	offFilter := resolveExemplarFilter("always-off")
	assert.Equal(t, exemplar.AlwaysOffFilter(bgCtx), offFilter(bgCtx))
	assert.Equal(t, exemplar.AlwaysOffFilter(sampledCtx), offFilter(sampledCtx))

	traceFilter := resolveExemplarFilter("trace-based")
	assert.Equal(t, exemplar.TraceBasedFilter(bgCtx), traceFilter(bgCtx))
	assert.Equal(t, exemplar.TraceBasedFilter(sampledCtx), traceFilter(sampledCtx))

	onFilter := resolveExemplarFilter("always-on")
	assert.Equal(t, exemplar.AlwaysOnFilter(bgCtx), onFilter(bgCtx))
	assert.Equal(t, exemplar.AlwaysOnFilter(sampledCtx), onFilter(sampledCtx))
}

func TestResolveExemplarFilterWithMetricsConfiguration(t *testing.T) {
	t.Parallel()

	defaultConfig := config.NewDefaultMetricsConfiguration()

	tests := []struct {
		name            string
		exemplarFilter  string
		expectExemplars bool
	}{
		{
			name:            "always-off filter with default config produces no exemplars",
			exemplarFilter:  "always-off",
			expectExemplars: false,
		},
		{
			name:            "trace-based filter with default config allows sampled exemplars",
			exemplarFilter:  "trace-based",
			expectExemplars: true,
		},
		{
			name:            "always-on filter with default config allows all exemplars",
			exemplarFilter:  "always-on",
			expectExemplars: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_ = defaultConfig
			filter := resolveExemplarFilter(tt.exemplarFilter)
			sampledCtx := sampledContext()
			assert.Equal(t, tt.expectExemplars, filter(sampledCtx))
		})
	}
}
