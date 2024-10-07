package config

import (
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func Test_metricsConfig_load(t *testing.T) {
	tests := []struct {
		name          string
		configMap     *corev1.ConfigMap
		expectedValue *metricsConfig
	}{
		{
			name: "Case 1: Test defaults",
			configMap: &corev1.ConfigMap{
				Data: map[string]string{},
			},
			expectedValue: &metricsConfig{
				metricsRefreshInterval: 0,
				namespaces:             namespacesConfig{IncludeNamespaces: []string{}, ExcludeNamespaces: []string{}},
				bucketBoundaries:       []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
				metricsExposure:        map[string]metricExposureConfig{},
			},
		},
		{
			name: "Case 2: All fields provided",
			configMap: &corev1.ConfigMap{
				Data: map[string]string{
					"metricsRefreshInterval": "10s",
					"namespaces":             `{"include": ["namespace1"], "exclude": ["namespace2"]}`,
					"bucketBoundaries":       "0.005, 0.01, 0.025, 0.05",
					"metricsExposure":        `{"metric1": {"enabled": true, "disabledLabelDimensions": ["dim1"]}, "metric2": {"enabled": true, "disabledLabelDimensions": ["dim1","dim2"], "bucketBoundaries": [0.025, 0.05]}}`,
				},
			},
			expectedValue: &metricsConfig{
				metricsRefreshInterval: 10 * time.Second,
				namespaces:             namespacesConfig{IncludeNamespaces: []string{"namespace1"}, ExcludeNamespaces: []string{"namespace2"}},
				bucketBoundaries:       []float64{0.005, 0.01, 0.025, 0.05},
				metricsExposure: map[string]metricExposureConfig{
					"metric1": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1"}, BucketBoundaries: []float64{0.005, 0.01, 0.025, 0.05}},
					"metric2": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1", "dim2"}, BucketBoundaries: []float64{0.025, 0.05}},
				},
			},
		},
		{
			name: "Case 3: Some of the fields provided",
			configMap: &corev1.ConfigMap{
				Data: map[string]string{
					"namespaces":      `{"include": ["namespace1"], "exclude": ["namespace2"]}`,
					"metricsExposure": `{"metric1": {"enabled": true, "disabledLabelDimensions": ["dim1"]}, "metric2": {"enabled": true, "disabledLabelDimensions": ["dim1","dim2"], "bucketBoundaries": [0.025, 0.05]}}`,
				},
			},
			expectedValue: &metricsConfig{
				metricsRefreshInterval: 0,
				namespaces:             namespacesConfig{IncludeNamespaces: []string{"namespace1"}, ExcludeNamespaces: []string{"namespace2"}},
				bucketBoundaries:       []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
				metricsExposure: map[string]metricExposureConfig{
					"metric1": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1"}, BucketBoundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30}},
					"metric2": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1", "dim2"}, BucketBoundaries: []float64{0.025, 0.05}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cd := NewDefaultMetricsConfiguration()
			cd.load(tt.configMap)

			if !reflect.DeepEqual(cd.metricsRefreshInterval, tt.expectedValue.metricsRefreshInterval) {
				t.Errorf("Expected %+v, but got %+v", tt.expectedValue.metricsRefreshInterval, cd.metricsRefreshInterval)
			}
			if !reflect.DeepEqual(cd.namespaces, tt.expectedValue.namespaces) {
				t.Errorf("Expected %+v, but got %+v", tt.expectedValue.namespaces, cd.namespaces)
			}
			if !reflect.DeepEqual(cd.bucketBoundaries, tt.expectedValue.bucketBoundaries) {
				t.Errorf("Expected %+v, but got %+v", tt.expectedValue.bucketBoundaries, cd.bucketBoundaries)
			}
			if !reflect.DeepEqual(cd.metricsExposure, tt.expectedValue.metricsExposure) {
				t.Errorf("Expected %+v, but got %+v", tt.expectedValue.metricsExposure, cd.metricsRefreshInterval)
			}
		})
	}
}

func Test_metricsConfig_BuildMeterProviderViews(t *testing.T) {
	tests := []struct {
		name            string
		metricsExposure map[string]metricExposureConfig
		expectedSize    int
		validateFunc    func([]sdkmetric.View) bool
	}{
		{
			name:            "Case 1: defaults",
			metricsExposure: map[string]metricExposureConfig{},
			expectedSize:    0,
		},
		{
			name: "Case 2: there is no matching entry on the exposure config",
			metricsExposure: map[string]metricExposureConfig{
				"metric1": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1"}, BucketBoundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30}},
			},
			expectedSize: 1,
			validateFunc: func(views []sdkmetric.View) bool {
				stream, _ := views[0](sdkmetric.Instrument{Name: "metric2"})
				assert := stream.AttributeFilter == nil
				assert = assert && stream.Aggregation == nil
				return assert
			},
		},
		{
			name: "Case 3: metrics enabled, no transformation configured",
			metricsExposure: map[string]metricExposureConfig{
				"metric1": {Enabled: ptr.To(true)},
			},
			expectedSize: 1,
			validateFunc: func(views []sdkmetric.View) bool {
				stream, _ := views[0](sdkmetric.Instrument{Name: "metric1"})
				assert := stream.AttributeFilter == nil
				assert = assert && stream.Aggregation == nil
				return assert
			},
		},
		{
			name: "Case 4: metrics enabled, histogram metric",
			metricsExposure: map[string]metricExposureConfig{
				"metric1": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1"}, BucketBoundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30}},
			},
			expectedSize: 1,
			validateFunc: func(views []sdkmetric.View) bool {
				stream, _ := views[0](sdkmetric.Instrument{Name: "metric1", Kind: sdkmetric.InstrumentKindHistogram})
				assert := stream.AttributeFilter(attribute.String("policy_validation_mode", ""))
				assert = assert && !stream.AttributeFilter(attribute.String("dim1", ""))
				assert = assert && reflect.DeepEqual(stream.Aggregation, sdkmetric.AggregationExplicitBucketHistogram{
					Boundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
					NoMinMax:   false,
				})
				return assert
			},
		},
		{
			name: "Case 5: metrics enabled, non histogram metric",
			metricsExposure: map[string]metricExposureConfig{
				"metric1": {Enabled: ptr.To(true), DisabledLabelDimensions: []string{"dim1"}, BucketBoundaries: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30}},
			},
			expectedSize: 1,
			validateFunc: func(views []sdkmetric.View) bool {
				stream, _ := views[0](sdkmetric.Instrument{Name: "metric1", Kind: sdkmetric.InstrumentKindCounter})
				assert := stream.AttributeFilter(attribute.String("policy_validation_mode", ""))
				assert = assert && !stream.AttributeFilter(attribute.String("dim1", ""))
				assert = assert && stream.Aggregation == nil
				return assert
			},
		},
		{
			name: "Case 6: metrics disabled",
			metricsExposure: map[string]metricExposureConfig{
				"metric1": {Enabled: ptr.To(false)},
			},
			expectedSize: 1,
			validateFunc: func(views []sdkmetric.View) bool {
				stream, _ := views[0](sdkmetric.Instrument{Name: "metric1"})
				return reflect.DeepEqual(stream.Aggregation, sdkmetric.AggregationDrop{})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcd := NewDefaultMetricsConfiguration()
			mcd.metricsExposure = tt.metricsExposure
			got := mcd.BuildMeterProviderViews()
			if len(got) != tt.expectedSize {
				t.Errorf("Expected result size to be %v, but got %v", tt.expectedSize, len(got))
			}
			if tt.validateFunc != nil {
				if !tt.validateFunc(got) {
					t.Errorf("The validation function did not return true!")
				}
			}
		})
	}
}

func Test_metricsConfig_GetBucketBoundaries(t *testing.T) {
	tests := []struct {
		name     string
		provided []float64
		want     []float64
	}{
		{
			name:     "Case 1: Test defaults",
			provided: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
			want:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 15, 20, 25, 30},
		},
		{
			name:     "Case 2: Custom",
			provided: []float64{0.005, 0.01, 0.025, 0.05},
			want:     []float64{0.005, 0.01, 0.025, 0.05},
		},
		{
			name:     "Case 3: Empty",
			provided: []float64{},
			want:     []float64{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcd := NewDefaultMetricsConfiguration()
			mcd.bucketBoundaries = tt.provided
			if got := mcd.GetBucketBoundaries(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetBucketBoundaries() = %v, want %v", got, tt.want)
			}
		})
	}
}
