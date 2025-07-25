package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// setupTestTracer sets up a proper OpenTelemetry tracer provider for testing
func setupTestTracer(t *testing.T) func() {
	// Create a new tracer provider with an in-memory exporter for testing
	tp := sdktrace.NewTracerProvider()

	// Store the original provider to restore it later
	originalProvider := otel.GetTracerProvider()

	// Set the test provider globally
	otel.SetTracerProvider(tp)

	// Return a cleanup function
	return func() {
		// Restore the original provider
		otel.SetTracerProvider(originalProvider)
	}
}

func TestNewSemanticTracer(t *testing.T) {
	st := NewSemanticTracer()
	require.NotNil(t, st)
	assert.NotNil(t, st.tracer)
}

func TestSemanticTracer_TracePolicy(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	opts := PolicySpanOptions{
		PolicyName:      "test-policy",
		PolicyNamespace: "default",
		PolicyType:      "ClusterPolicy",
		RuleName:        "test-rule",
		Operation:       "validate",
	}

	var spanCalled bool
	st.TracePolicy(ctx, opts, func(ctx context.Context, span trace.Span) {
		spanCalled = true
		assert.NotNil(t, span)
		assert.True(t, span.IsRecording())
	})

	assert.True(t, spanCalled)
}

func TestSemanticTracer_TraceRule(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	var spanCalled bool
	st.TraceRule(ctx, "validate-labels", "validation", func(ctx context.Context, span trace.Span) {
		spanCalled = true
		assert.NotNil(t, span)
		assert.True(t, span.IsRecording())
	})

	assert.True(t, spanCalled)
}

func TestSemanticTracer_TraceResource(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	var spanCalled bool
	st.TraceResource(ctx, "Pod", "test-pod", "default", func(ctx context.Context, span trace.Span) {
		spanCalled = true
		assert.NotNil(t, span)
		assert.True(t, span.IsRecording())
	})

	assert.True(t, spanCalled)
}

func TestSemanticTracer_TraceCacheOperation(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	testCases := []struct {
		operation string
		cacheType string
		key       string
		hit       bool
	}{
		{"get", "policy", "default/test-policy", true},
		{"set", "resource", "pod/test", false},
		{"delete", "binding", "user/admin", true},
	}

	for _, tc := range testCases {
		t.Run(tc.operation+"_"+tc.cacheType, func(t *testing.T) {
			var spanCalled bool
			st.TraceCacheOperation(ctx, tc.operation, tc.cacheType, tc.key, tc.hit, func(ctx context.Context, span trace.Span) {
				spanCalled = true
				assert.NotNil(t, span)
				assert.True(t, span.IsRecording())
			})

			assert.True(t, spanCalled)
		})
	}
}

func TestSemanticTracer_TraceAdmissionRequest(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	var spanCalled bool
	st.TraceAdmissionRequest(ctx, "test-uid", "CREATE", "Pod", func(ctx context.Context, span trace.Span) {
		spanCalled = true
		assert.NotNil(t, span)
		assert.True(t, span.IsRecording())
	})

	assert.True(t, spanCalled)
}

func TestSemanticTracer_TraceValidation(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	t.Run("successful validation", func(t *testing.T) {
		var spanCalled bool
		err := st.TraceValidation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return nil
		})

		assert.True(t, spanCalled)
		assert.NoError(t, err)
	})

	t.Run("failed validation", func(t *testing.T) {
		var spanCalled bool
		testErr := errors.New("validation failed")

		err := st.TraceValidation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return testErr
		})

		assert.True(t, spanCalled)
		assert.Equal(t, testErr, err)
	})
}

func TestSemanticTracer_TraceMutation(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	t.Run("successful mutation", func(t *testing.T) {
		var spanCalled bool
		err := st.TraceMutation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return nil
		})

		assert.True(t, spanCalled)
		assert.NoError(t, err)
	})

	t.Run("failed mutation", func(t *testing.T) {
		var spanCalled bool
		testErr := errors.New("mutation failed")

		err := st.TraceMutation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return testErr
		})

		assert.True(t, spanCalled)
		assert.Equal(t, testErr, err)
	})
}

func TestSemanticTracer_TraceGeneration(t *testing.T) {
	cleanup := setupTestTracer(t)
	defer cleanup()

	st := NewSemanticTracer()
	ctx := context.Background()

	t.Run("successful generation", func(t *testing.T) {
		var spanCalled bool
		err := st.TraceGeneration(ctx, "NetworkPolicy", "default-deny", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return nil
		})

		assert.True(t, spanCalled)
		assert.NoError(t, err)
	})

	t.Run("failed generation", func(t *testing.T) {
		var spanCalled bool
		testErr := errors.New("generation failed")

		err := st.TraceGeneration(ctx, "ConfigMap", "test-config", func(ctx context.Context, span trace.Span) error {
			spanCalled = true
			assert.NotNil(t, span)
			assert.True(t, span.IsRecording())
			return testErr
		})

		assert.True(t, spanCalled)
		assert.Equal(t, testErr, err)
	})
}

func TestSemanticTracer_AddEventToSpan(t *testing.T) {
	st := NewSemanticTracer()
	tracer := otel.GetTracerProvider().Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Should not panic
	st.AddEventToSpan(ctx, "test.event",
		attribute.String("key", "value"),
		attribute.Int("count", 42),
	)
}

func TestSemanticTracer_SetSpanAttribute(t *testing.T) {
	st := NewSemanticTracer()
	tracer := otel.GetTracerProvider().Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	// Test different attribute types
	testCases := []struct {
		key   string
		value interface{}
	}{
		{"string_attr", "test-value"},
		{"int_attr", 42},
		{"int64_attr", int64(1234)},
		{"bool_attr", true},
		{"float64_attr", 3.14},
		{"other_attr", []string{"a", "b"}},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			// Should not panic
			st.SetSpanAttribute(ctx, tc.key, tc.value)
		})
	}
}

func TestSemanticTracer_RecordEvent(t *testing.T) {
	st := NewSemanticTracer()
	tracer := otel.GetTracerProvider().Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	attributes := map[string]interface{}{
		"string_attr":  "test-value",
		"int_attr":     42,
		"int64_attr":   int64(1234),
		"bool_attr":    true,
		"float64_attr": 3.14,
		"other_attr":   []string{"a", "b"},
	}

	// Should not panic
	st.RecordEvent(ctx, "test.event", "Test event message", attributes)
}

func TestPolicySpanOptions(t *testing.T) {
	opts := PolicySpanOptions{
		PolicyName:      "test-policy",
		PolicyNamespace: "kyverno",
		PolicyType:      "ClusterPolicy",
		RuleName:        "validate-labels",
		Operation:       "validate",
	}

	assert.Equal(t, "test-policy", opts.PolicyName)
	assert.Equal(t, "kyverno", opts.PolicyNamespace)
	assert.Equal(t, "ClusterPolicy", opts.PolicyType)
	assert.Equal(t, "validate-labels", opts.RuleName)
	assert.Equal(t, "validate", opts.Operation)
}

func TestSemanticSpanConstants(t *testing.T) {
	// Test that all span name constants are properly defined
	assert.Equal(t, "policy.processing", PolicyProcessingSpan)
	assert.Equal(t, "rule.evaluation", RuleEvaluationSpan)
	assert.Equal(t, "resource.processing", ResourceProcessingSpan)
	assert.Equal(t, "cache.operation", CacheOperationSpan)
	assert.Equal(t, "admission.request", AdmissionRequestSpan)
	assert.Equal(t, "validation.check", ValidationSpan)
	assert.Equal(t, "mutation.apply", MutationSpan)
	assert.Equal(t, "generation.create", GenerationSpan)
}

func BenchmarkSemanticTracer_TracePolicy(b *testing.B) {
	st := NewSemanticTracer()
	ctx := context.Background()

	opts := PolicySpanOptions{
		PolicyName:      "test-policy",
		PolicyNamespace: "default",
		PolicyType:      "ClusterPolicy",
		RuleName:        "test-rule",
		Operation:       "validate",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.TracePolicy(ctx, opts, func(ctx context.Context, span trace.Span) {
			// Minimal work in the span
		})
	}
}

func BenchmarkSemanticTracer_TraceValidation(b *testing.B) {
	st := NewSemanticTracer()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = st.TraceValidation(ctx, "admission", func(ctx context.Context, span trace.Span) error {
			return nil
		})
	}
}

func BenchmarkSemanticTracer_SetSpanAttribute(b *testing.B) {
	st := NewSemanticTracer()
	tracer := otel.GetTracerProvider().Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.SetSpanAttribute(ctx, "test_attr", "test_value")
	}
}
