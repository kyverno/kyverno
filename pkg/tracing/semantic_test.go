package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestNewSemanticTracer(t *testing.T) {
	tests := []struct {
		name           string
		setupProvider  func() *trace.TracerProvider
		expectNoOpMode bool
	}{
		{
			name: "valid tracer provider",
			setupProvider: func() *trace.TracerProvider {
				return trace.NewTracerProvider()
			},
			expectNoOpMode: false,
		},
		{
			name: "nil tracer provider",
			setupProvider: func() *trace.TracerProvider {
				return nil
			},
			expectNoOpMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			if provider != nil {
				otel.SetTracerProvider(provider)
				defer func() {
					if err := provider.Shutdown(context.Background()); err != nil {
						t.Logf("Failed to shutdown tracer provider: %v", err)
					}
				}()
			} else {
				otel.SetTracerProvider(nil)
			}

			st := NewSemanticTracer()
			if st == nil {
				t.Error("NewSemanticTracer() returned nil")
			}

			if tt.expectNoOpMode && st.tracer != nil {
				t.Error("Expected no-op mode but got tracer")
			}
		})
	}
}

func TestSemanticTracer_TracePolicy(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name string
		opts PolicySpanOptions
	}{
		{
			name: "validation policy",
			opts: PolicySpanOptions{
				PolicyName:      "test-policy",
				PolicyNamespace: "default",
				Operation:       "validate",
				PolicyType:      "ClusterPolicy",
			},
		},
		{
			name: "mutation policy",
			opts: PolicySpanOptions{
				PolicyName:      "mutate-policy",
				PolicyNamespace: "",
				Operation:       "mutate",
				PolicyType:      "ClusterPolicy",
			},
		},
		{
			name: "namespaced policy",
			opts: PolicySpanOptions{
				PolicyName:      "ns-policy",
				PolicyNamespace: "test-namespace",
				Operation:       "generate",
				PolicyType:      "Policy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			st.TracePolicy(ctx, tt.opts, func(ctx context.Context, span oteltrace.Span) {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
				// Verify span is active
				if oteltrace.SpanFromContext(ctx) == nil {
					t.Error("Expected span to be active in context")
				}
			})

			if !called {
				t.Error("Function was not called")
			}
		})
	}
}

func TestSemanticTracer_TraceRule(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name     string
		ruleName string
		ruleType string
	}{
		{
			name:     "validation rule",
			ruleName: "check-labels",
			ruleType: "validation",
		},
		{
			name:     "mutation rule",
			ruleName: "add-labels",
			ruleType: "mutation",
		},
		{
			name:     "generation rule",
			ruleName: "create-secret",
			ruleType: "generation",
		},
		{
			name:     "image verification rule",
			ruleName: "verify-image",
			ruleType: "imageVerification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			st.TraceRule(ctx, tt.ruleName, tt.ruleType, func(ctx context.Context, span oteltrace.Span) {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
			})

			if !called {
				t.Error("Function was not called")
			}
		})
	}
}

func TestSemanticTracer_TraceResource(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name              string
		resourceKind      string
		resourceName      string
		resourceNamespace string
	}{
		{
			name:              "pod resource",
			resourceKind:      "Pod",
			resourceName:      "test-pod",
			resourceNamespace: "default",
		},
		{
			name:              "cluster resource",
			resourceKind:      "ClusterRole",
			resourceName:      "test-role",
			resourceNamespace: "",
		},
		{
			name:              "deployment resource",
			resourceKind:      "Deployment",
			resourceName:      "app-deployment",
			resourceNamespace: "production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			st.TraceResource(ctx, tt.resourceKind, tt.resourceName, tt.resourceNamespace, func(ctx context.Context, span oteltrace.Span) {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
			})

			if !called {
				t.Error("Function was not called")
			}
		})
	}
}

func TestSemanticTracer_TraceCacheOperation(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name      string
		operation string
		cacheType string
		key       string
		hit       bool
	}{
		{
			name:      "cache hit",
			operation: "get",
			cacheType: "policy",
			key:       "policy-123",
			hit:       true,
		},
		{
			name:      "cache miss",
			operation: "get",
			cacheType: "validation",
			key:       "validation-456",
			hit:       false,
		},
		{
			name:      "cache put",
			operation: "put",
			cacheType: "image",
			key:       "image-789",
			hit:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			st.TraceCacheOperation(ctx, tt.operation, tt.cacheType, tt.key, tt.hit, func(ctx context.Context, span oteltrace.Span) {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
			})

			if !called {
				t.Error("Function was not called")
			}
		})
	}
}

func TestSemanticTracer_TraceAdmissionRequest(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name         string
		requestUID   string
		operation    string
		resourceKind string
	}{
		{
			name:         "create pod",
			requestUID:   "req-123",
			operation:    "CREATE",
			resourceKind: "Pod",
		},
		{
			name:         "update deployment",
			requestUID:   "req-456",
			operation:    "UPDATE",
			resourceKind: "Deployment",
		},
		{
			name:         "delete service",
			requestUID:   "req-789",
			operation:    "DELETE",
			resourceKind: "Service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			st.TraceAdmissionRequest(ctx, tt.requestUID, tt.operation, tt.resourceKind, func(ctx context.Context, span oteltrace.Span) {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
			})

			if !called {
				t.Error("Function was not called")
			}
		})
	}
}

func TestSemanticTracer_TraceValidation(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name           string
		validationType string
		shouldError    bool
	}{
		{
			name:           "schema validation success",
			validationType: "schema",
			shouldError:    false,
		},
		{
			name:           "policy validation success",
			validationType: "policy",
			shouldError:    false,
		},
		{
			name:           "validation with error",
			validationType: "custom",
			shouldError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			expectedError := "validation failed"

			err := st.TraceValidation(ctx, tt.validationType, func(ctx context.Context, span oteltrace.Span) error {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
				if tt.shouldError {
					return &ValidationError{Message: expectedError}
				}
				return nil
			})

			if !called {
				t.Error("Function was not called")
			}

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if err.Error() != expectedError {
					t.Errorf("Expected error %q but got %q", expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

func TestSemanticTracer_TraceMutation(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name         string
		mutationType string
		shouldError  bool
	}{
		{
			name:         "label mutation success",
			mutationType: "labels",
			shouldError:  false,
		},
		{
			name:         "annotation mutation success",
			mutationType: "annotations",
			shouldError:  false,
		},
		{
			name:         "mutation with error",
			mutationType: "custom",
			shouldError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			expectedError := "mutation failed"

			err := st.TraceMutation(ctx, tt.mutationType, func(ctx context.Context, span oteltrace.Span) error {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
				if tt.shouldError {
					return &MutationError{Message: expectedError}
				}
				return nil
			})

			if !called {
				t.Error("Function was not called")
			}

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if err.Error() != expectedError {
					t.Errorf("Expected error %q but got %q", expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

func TestSemanticTracer_TraceGeneration(t *testing.T) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()

	tests := []struct {
		name        string
		targetKind  string
		targetName  string
		shouldError bool
	}{
		{
			name:        "secret generation success",
			targetKind:  "Secret",
			targetName:  "generated-secret",
			shouldError: false,
		},
		{
			name:        "configmap generation success",
			targetKind:  "ConfigMap",
			targetName:  "generated-cm",
			shouldError: false,
		},
		{
			name:        "generation with error",
			targetKind:  "Role",
			targetName:  "generated-role",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			expectedError := "generation failed"

			err := st.TraceGeneration(ctx, tt.targetKind, tt.targetName, func(ctx context.Context, span oteltrace.Span) error {
				called = true
				if span == nil {
					t.Error("Expected non-nil span")
				}
				if tt.shouldError {
					return &GenerationError{Message: expectedError}
				}
				return nil
			})

			if !called {
				t.Error("Function was not called")
			}

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if err.Error() != expectedError {
					t.Errorf("Expected error %q but got %q", expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error but got %v", err)
			}
		})
	}
}

func TestSemanticTracer_NoOpMode(t *testing.T) {
	// Test behavior when no tracer provider is available
	otel.SetTracerProvider(nil)

	st := NewSemanticTracer()
	if st == nil {
		t.Fatal("NewSemanticTracer() returned nil")
	}

	if st.tracer != nil {
		t.Error("Expected nil tracer in no-op mode")
	}

	ctx := context.Background()

	// All these should work without panicking in no-op mode
	called := false

	st.TracePolicy(ctx, PolicySpanOptions{
		PolicyName: "test",
		Operation:  "validate",
	}, func(ctx context.Context, span oteltrace.Span) {
		called = true
	})
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	st.TraceRule(ctx, "test-rule", "validation", func(ctx context.Context, span oteltrace.Span) {
		called = true
	})
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	st.TraceResource(ctx, "Pod", "test", "default", func(ctx context.Context, span oteltrace.Span) {
		called = true
	})
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	st.TraceCacheOperation(ctx, "get", "policy", "key", true, func(ctx context.Context, span oteltrace.Span) {
		called = true
	})
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	st.TraceAdmissionRequest(ctx, "req-123", "CREATE", "Pod", func(ctx context.Context, span oteltrace.Span) {
		called = true
	})
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	// Test error-returning methods
	err := st.TraceValidation(ctx, "test", func(ctx context.Context, span oteltrace.Span) error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error in no-op mode, got %v", err)
	}
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	err = st.TraceMutation(ctx, "test", func(ctx context.Context, span oteltrace.Span) error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error in no-op mode, got %v", err)
	}
	if !called {
		t.Error("Function should be called even in no-op mode")
	}

	called = false
	err = st.TraceGeneration(ctx, "Secret", "test", func(ctx context.Context, span oteltrace.Span) error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("Expected no error in no-op mode, got %v", err)
	}
	if !called {
		t.Error("Function should be called even in no-op mode")
	}
}

// Helper error types for testing
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type MutationError struct {
	Message string
}

func (e *MutationError) Error() string {
	return e.Message
}

type GenerationError struct {
	Message string
}

func (e *GenerationError) Error() string {
	return e.Message
}

// BenchmarkSemanticTracer tests the performance impact of tracing
func BenchmarkSemanticTracer_TracePolicy(b *testing.B) {
	provider := trace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	defer func() {
		if err := provider.Shutdown(context.Background()); err != nil {
			b.Logf("Failed to shutdown tracer provider: %v", err)
		}
	}()

	st := NewSemanticTracer()
	ctx := context.Background()
	opts := PolicySpanOptions{
		PolicyName: "benchmark-policy",
		Operation:  "validate",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.TracePolicy(ctx, opts, func(ctx context.Context, span oteltrace.Span) {
			// Minimal work
		})
	}
}

func BenchmarkSemanticTracer_NoOpMode(b *testing.B) {
	// Test performance in no-op mode
	otel.SetTracerProvider(nil)

	st := NewSemanticTracer()
	ctx := context.Background()
	opts := PolicySpanOptions{
		PolicyName: "benchmark-policy",
		Operation:  "validate",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.TracePolicy(ctx, opts, func(ctx context.Context, span oteltrace.Span) {
			// Minimal work
		})
	}
}
