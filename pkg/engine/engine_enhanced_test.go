package engine

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewEnhancedPolicyProcessor(t *testing.T) {
	logger := logr.Discard()

	// Setup test meter provider for ResourceMetrics
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)

	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	if processor == nil {
		t.Error("Expected processor to be non-nil")
	}

	if processor.logger != logger {
		t.Error("Expected logger to be set correctly")
	}

	if processor.resourceMetrics != resourceMetrics {
		t.Error("Expected resourceMetrics to be set correctly")
	}
}

func TestEnhancedPolicyProcessor_ProcessPolicyWithEnhancedTracing(t *testing.T) {
	// Setup test tracer
	traceProvider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(traceProvider)

	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)

	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	// Create a test policy
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Enforce,
			Rules: []kyvernov1.Rule{
				{
					Name: "test-rule",
					MatchResources: kyvernov1.MatchResources{
						Any: kyvernov1.ResourceFilters{
							{
								ResourceDescription: kyvernov1.ResourceDescription{
									Kinds: []string{"Pod"},
								},
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	resource := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
		},
	}

	// Test the enhanced tracing
	err = processor.ProcessPolicyWithEnhancedTracing(ctx, policy, resource)
	if err != nil {
		t.Errorf("ProcessPolicyWithEnhancedTracing() returned error: %v", err)
	}
}

func TestEnhancedPolicyProcessor_ProcessResourceWithEnhancedTracing(t *testing.T) {
	// Setup test tracer
	traceProvider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(traceProvider)

	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)

	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	ctx := context.Background()

	tests := []struct {
		testName          string
		resourceKind      string
		resourceName      string
		resourceNamespace string
	}{
		{
			testName:          "pod resource",
			resourceKind:      "Pod",
			resourceName:      "test-pod",
			resourceNamespace: "default",
		},
		{
			testName:          "service resource",
			resourceKind:      "Service",
			resourceName:      "test-service",
			resourceNamespace: "kube-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := processor.ProcessResourceWithEnhancedTracing(
				ctx,
				tt.resourceKind,
				tt.resourceName,
				tt.resourceNamespace,
			)
			if err != nil {
				t.Errorf("ProcessResourceWithEnhancedTracing() returned error: %v", err)
			}
		})
	}
}

func TestEnhancedPolicyProcessor_ProcessAdmissionRequestWithEnhancedTracing(t *testing.T) {
	// Setup test tracer
	traceProvider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(traceProvider)

	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)

	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		t.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	ctx := context.Background()

	tests := []struct {
		testName  string
		user      string
		groups    []string
		operation string
		dryRun    bool
	}{
		{
			testName:  "create pod admission",
			user:      "test-user",
			groups:    []string{"system:authenticated"},
			operation: "CREATE",
			dryRun:    false,
		},
		{
			testName:  "update service admission",
			user:      "admin",
			groups:    []string{"system:masters"},
			operation: "UPDATE",
			dryRun:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			err := processor.ProcessAdmissionRequestWithEnhancedTracing(
				ctx,
				tt.user,
				tt.groups,
				tt.operation,
				tt.dryRun,
			)
			if err != nil {
				t.Errorf("ProcessAdmissionRequestWithEnhancedTracing() returned error: %v", err)
			}
		})
	}
}

// BenchmarkEnhancedPolicyProcessor tests the performance impact of enhanced tracing
func BenchmarkEnhancedPolicyProcessor_ProcessResourceWithEnhancedTracing(b *testing.B) {
	// Setup test tracer
	traceProvider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(traceProvider)

	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)

	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.ProcessResourceWithEnhancedTracing(
			ctx,
			"Pod",
			"test-pod",
			"default",
		)
	}
}

func BenchmarkEnhancedPolicyProcessor_ProcessAdmissionRequestWithEnhancedTracing(b *testing.B) {
	// Setup test tracer
	traceProvider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(traceProvider)

	// Setup test meter provider
	reader := sdkmetric.NewManualReader()
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(meterProvider)

	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	if err != nil {
		b.Fatalf("Failed to create ResourceMetrics: %v", err)
	}

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = processor.ProcessAdmissionRequestWithEnhancedTracing(
			ctx,
			"test-user",
			[]string{"system:authenticated"},
			"CREATE",
			false,
		)
	}
}
