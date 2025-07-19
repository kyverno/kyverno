package engine

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnhancedPolicyProcessor(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	require.NotNil(t, processor)

	assert.Equal(t, logger, processor.logger)
	assert.Equal(t, resourceMetrics, processor.resourceMetrics)
	assert.NotNil(t, processor.semanticTracer)
}

func TestEnhancedPolicyProcessor_ProcessPolicy(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	request := PolicyRequest{
		PolicyName:        "test-policy",
		PolicyNamespace:   "default",
		PolicyType:        "ClusterPolicy",
		ResourceKind:      "Pod",
		ResourceName:      "test-pod",
		ResourceNamespace: "default",
		Operation:         "CREATE",
		RequestUID:        "test-uid-123",
	}

	result, err := processor.ProcessPolicy(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Allowed)
	assert.True(t, result.Applied)
	assert.True(t, result.Duration > 0)
	assert.Greater(t, len(result.Violations), 0) // Should have at least one violation from the test
}

func TestEnhancedPolicyProcessor_ProcessPolicy_ValidationViolations(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	request := PolicyRequest{
		PolicyName:        "validation-policy",
		PolicyNamespace:   "kyverno",
		PolicyType:        "ClusterPolicy",
		ResourceKind:      "Deployment",
		ResourceName:      "test-deployment",
		ResourceNamespace: "default",
		Operation:         "CREATE",
		RequestUID:        "test-uid-456",
	}

	result, err := processor.ProcessPolicy(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should still be allowed even with violations (policy is in report mode)
	assert.True(t, result.Allowed)
	assert.Contains(t, result.Violations, "Resource limits not specified")
}

func TestEnhancedPolicyProcessor_ProcessPolicy_GenerationRule(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	// Test namespace creation which should trigger generation
	request := PolicyRequest{
		PolicyName:        "generation-policy",
		PolicyNamespace:   "kyverno",
		PolicyType:        "ClusterPolicy",
		ResourceKind:      "Namespace",
		ResourceName:      "test-namespace",
		ResourceNamespace: "",
		Operation:         "CREATE",
		RequestUID:        "test-uid-789",
	}

	result, err := processor.ProcessPolicy(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Allowed)
	assert.True(t, result.Applied)
}

func TestEnhancedPolicyProcessor_ProcessBatch(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	requests := []PolicyRequest{
		{
			PolicyName:        "policy-1",
			PolicyNamespace:   "default",
			PolicyType:        "ClusterPolicy",
			ResourceKind:      "Pod",
			ResourceName:      "test-pod-1",
			ResourceNamespace: "default",
			Operation:         "CREATE",
			RequestUID:        "test-uid-1",
		},
		{
			PolicyName:        "policy-2",
			PolicyNamespace:   "default",
			PolicyType:        "ClusterPolicy",
			ResourceKind:      "Service",
			ResourceName:      "test-service-1",
			ResourceNamespace: "default",
			Operation:         "UPDATE",
			RequestUID:        "test-uid-2",
		},
		{
			PolicyName:        "policy-3",
			PolicyNamespace:   "kyverno",
			PolicyType:        "ClusterPolicy",
			ResourceKind:      "Namespace",
			ResourceName:      "test-namespace-1",
			ResourceNamespace: "",
			Operation:         "CREATE",
			RequestUID:        "test-uid-3",
		},
	}

	results, err := processor.ProcessBatch(ctx, requests)
	require.NoError(t, err)
	require.NotNil(t, results)
	require.Len(t, results, 3)

	for i, result := range results {
		assert.True(t, result.Allowed, "Request %d should be allowed", i)
		assert.True(t, result.Duration > 0, "Request %d should have processing duration", i)
	}
}

func TestEnhancedPolicyProcessor_GetMetrics(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	metrics := processor.GetMetrics()
	assert.Equal(t, resourceMetrics, metrics)
}

func TestEnhancedPolicyProcessor_GetTracer(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)

	tracer := processor.GetTracer()
	assert.NotNil(t, tracer)
	assert.Equal(t, processor.semanticTracer, tracer)
}

func TestPolicyRequest_Validation(t *testing.T) {
	request := PolicyRequest{
		PolicyName:        "test-policy",
		PolicyNamespace:   "default",
		PolicyType:        "ClusterPolicy",
		ResourceKind:      "Pod",
		ResourceName:      "test-pod",
		ResourceNamespace: "default",
		Operation:         "CREATE",
		RequestUID:        "test-uid",
	}

	assert.Equal(t, "test-policy", request.PolicyName)
	assert.Equal(t, "default", request.PolicyNamespace)
	assert.Equal(t, "ClusterPolicy", request.PolicyType)
	assert.Equal(t, "Pod", request.ResourceKind)
	assert.Equal(t, "test-pod", request.ResourceName)
	assert.Equal(t, "default", request.ResourceNamespace)
	assert.Equal(t, "CREATE", request.Operation)
	assert.Equal(t, "test-uid", request.RequestUID)
}

func TestPolicyResult_Validation(t *testing.T) {
	result := PolicyResult{
		Allowed:    true,
		Message:    "Policy processed successfully",
		Violations: []string{"violation1", "violation2"},
		Applied:    true,
		Duration:   100 * time.Millisecond,
	}

	assert.True(t, result.Allowed)
	assert.Equal(t, "Policy processed successfully", result.Message)
	assert.Len(t, result.Violations, 2)
	assert.Contains(t, result.Violations, "violation1")
	assert.Contains(t, result.Violations, "violation2")
	assert.True(t, result.Applied)
	assert.Equal(t, 100*time.Millisecond, result.Duration)
}

func TestEnhancedPolicyProcessor_DifferentResourceTypes(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	resourceTypes := []string{"Pod", "Deployment", "Service", "ConfigMap", "Secret", "Namespace"}

	for _, resourceType := range resourceTypes {
		t.Run(resourceType, func(t *testing.T) {
			request := PolicyRequest{
				PolicyName:        "multi-resource-policy",
				PolicyNamespace:   "default",
				PolicyType:        "ClusterPolicy",
				ResourceKind:      resourceType,
				ResourceName:      "test-" + resourceType,
				ResourceNamespace: "default",
				Operation:         "CREATE",
				RequestUID:        "test-uid-" + resourceType,
			}

			result, err := processor.ProcessPolicy(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.True(t, result.Allowed)
			assert.True(t, result.Duration > 0)
		})
	}
}

func TestEnhancedPolicyProcessor_DifferentOperations(t *testing.T) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(t, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	operations := []string{"CREATE", "UPDATE", "DELETE"}

	for _, operation := range operations {
		t.Run(operation, func(t *testing.T) {
			request := PolicyRequest{
				PolicyName:        "operation-policy",
				PolicyNamespace:   "default",
				PolicyType:        "ClusterPolicy",
				ResourceKind:      "Pod",
				ResourceName:      "test-pod",
				ResourceNamespace: "default",
				Operation:         operation,
				RequestUID:        "test-uid-" + operation,
			}

			result, err := processor.ProcessPolicy(ctx, request)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.True(t, result.Allowed)
			assert.True(t, result.Duration > 0)
		})
	}
}

func BenchmarkEnhancedPolicyProcessor_ProcessPolicy(b *testing.B) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(b, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	request := PolicyRequest{
		PolicyName:        "benchmark-policy",
		PolicyNamespace:   "default",
		PolicyType:        "ClusterPolicy",
		ResourceKind:      "Pod",
		ResourceName:      "test-pod",
		ResourceNamespace: "default",
		Operation:         "CREATE",
		RequestUID:        "test-uid-benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessPolicy(ctx, request)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEnhancedPolicyProcessor_ProcessBatch(b *testing.B) {
	logger := logr.Discard()
	resourceMetrics, err := metrics.NewResourceMetrics(logger)
	require.NoError(b, err)

	processor := NewEnhancedPolicyProcessor(logger, resourceMetrics)
	ctx := context.Background()

	requests := make([]PolicyRequest, 10)
	for i := 0; i < 10; i++ {
		requests[i] = PolicyRequest{
			PolicyName:        "batch-policy",
			PolicyNamespace:   "default",
			PolicyType:        "ClusterPolicy",
			ResourceKind:      "Pod",
			ResourceName:      "test-pod",
			ResourceNamespace: "default",
			Operation:         "CREATE",
			RequestUID:        "test-uid-batch",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ProcessBatch(ctx, requests)
		if err != nil {
			b.Fatal(err)
		}
	}
}
