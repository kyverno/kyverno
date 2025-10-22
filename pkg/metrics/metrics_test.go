package metrics

import (
	"reflect"
	"testing"
	"time"

	kconfig "github.com/kyverno/kyverno/pkg/config"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
