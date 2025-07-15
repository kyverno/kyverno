package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestTracePolicy(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	options := PolicySpanOptions{
		PolicyName:      "test-policy",
		PolicyNamespace: "default",
		PolicyType:      "cluster",
		ValidationMode:  "enforce",
		BackgroundMode:  "true",
	}

	called := false
	TracePolicy(ctx, SpanPolicyValidation, options, func(ctx context.Context, span trace.Span) {
		called = true
		// Verify span is active
		if !span.IsRecording() {
			t.Error("Expected span to be recording")
		}
	})

	if !called {
		t.Error("Expected function to be called")
	}
}

func TestTraceRule(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	options := RuleSpanOptions{
		RuleName:       "test-rule",
		RuleType:       "validation",
		ExecutionCause: "admission",
	}

	called := false
	TraceRule(ctx, SpanRuleExecution, options, func(ctx context.Context, span trace.Span) {
		called = true
		// Verify span is active
		if !span.IsRecording() {
			t.Error("Expected span to be recording")
		}
	})

	if !called {
		t.Error("Expected function to be called")
	}
}

func TestTraceResource(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	options := ResourceSpanOptions{
		Kind:      "Pod",
		Name:      "test-pod",
		Namespace: "default",
		Version:   "v1",
		Operation: "CREATE",
	}

	called := false
	TraceResource(ctx, SpanResourceProcess, options, func(ctx context.Context, span trace.Span) {
		called = true
		// Verify span is active
		if !span.IsRecording() {
			t.Error("Expected span to be recording")
		}
	})

	if !called {
		t.Error("Expected function to be called")
	}
}

func TestTraceCacheOperation(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	cacheKey := "test-cache-key"

	tests := []struct {
		name      string
		operation string
		cacheKey  string
		function  func(context.Context, trace.Span) (bool, error)
		expectHit bool
		expectErr bool
	}{
		{
			name:      "cache hit",
			operation: "get",
			cacheKey:  cacheKey,
			function: func(ctx context.Context, span trace.Span) (bool, error) {
				return true, nil
			},
			expectHit: true,
			expectErr: false,
		},
		{
			name:      "cache miss",
			operation: "get",
			cacheKey:  cacheKey,
			function: func(ctx context.Context, span trace.Span) (bool, error) {
				return false, nil
			},
			expectHit: false,
			expectErr: false,
		},
		{
			name:      "cache error",
			operation: "get",
			cacheKey:  cacheKey,
			function: func(ctx context.Context, span trace.Span) (bool, error) {
				// Simulate an error
				return false, context.DeadlineExceeded
			},
			expectHit: false,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, err := TraceCacheOperation(ctx, tt.operation, tt.cacheKey, tt.function)

			if hit != tt.expectHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.expectHit, hit)
			}

			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error=%v, got error=%v", tt.expectErr, err != nil)
			}
		})
	}
}

func TestAddEnginePhase(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	tests := []struct {
		name  string
		phase string
	}{
		{
			name:  "validation phase",
			phase: "validation",
		},
		{
			name:  "mutation phase",
			phase: "mutation",
		},
		{
			name:  "generation phase",
			phase: "generation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddEnginePhase(ctx, tt.phase)
			// Test passes if no panic occurs
		})
	}
}

func TestAddEngineResult(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	tests := []struct {
		name   string
		result string
	}{
		{
			name:   "success result",
			result: "success",
		},
		{
			name:   "fail result",
			result: "fail",
		},
		{
			name:   "skip result",
			result: "skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddEngineResult(ctx, tt.result)
			// Test passes if no panic occurs
		})
	}
}

func TestRecordRuleResult(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	tests := []struct {
		name     string
		ruleName string
		result   string
	}{
		{
			name:     "rule pass",
			ruleName: "test-rule",
			result:   "pass",
		},
		{
			name:     "rule fail",
			ruleName: "test-rule",
			result:   "fail",
		},
		{
			name:     "rule skip",
			ruleName: "test-rule",
			result:   "skip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordRuleResult(ctx, tt.ruleName, tt.result)
			// Test passes if no panic occurs
		})
	}
}

func TestRecordPolicyViolation(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	tests := []struct {
		name       string
		policyName string
		ruleName   string
		message    string
	}{
		{
			name:       "high severity violation",
			policyName: "test-policy",
			ruleName:   "test-rule",
			message:    "Policy violation detected",
		},
		{
			name:       "medium severity violation",
			policyName: "test-policy",
			ruleName:   "test-rule",
			message:    "Warning: potential issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RecordPolicyViolation(ctx, tt.policyName, tt.ruleName, tt.message)
			// Test passes if no panic occurs
		})
	}
}

func TestTraceRequest(t *testing.T) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	options := RequestSpanOptions{
		Operation: "CREATE",
		User:      "test-user",
		Groups:    []string{"system:authenticated"},
		DryRun:    false,
	}

	called := false
	TraceRequest(ctx, SpanWebhookAdmission, options, func(ctx context.Context, span trace.Span) {
		called = true
		// Verify span is active
		if !span.IsRecording() {
			t.Error("Expected span to be recording")
		}
	})

	if !called {
		t.Error("Expected function to be called")
	}
}

// BenchmarkTracePolicy tests the performance impact of tracing
func BenchmarkTracePolicy(b *testing.B) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()
	options := PolicySpanOptions{
		PolicyName:      "test-policy",
		PolicyNamespace: "default",
		PolicyType:      "cluster",
		ValidationMode:  "enforce",
		BackgroundMode:  "true",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TracePolicy(ctx, SpanPolicyValidation, options, func(ctx context.Context, span trace.Span) {
			// Minimal work
			span.SetStatus(codes.Ok, "")
		})
	}
}

func BenchmarkTraceCacheOperation(b *testing.B) {
	// Setup test tracer
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = TraceCacheOperation(ctx, "get", "test-key", func(ctx context.Context, span trace.Span) (bool, error) {
			return true, nil
		})
	}
}

// TestAttributeKeys verifies that all attribute keys are properly defined
func TestAttributeKeys(t *testing.T) {
	tests := []struct {
		name string
		key  attribute.Key
	}{
		{"RuleTypeKey", RuleTypeKey},
		{"RuleResultKey", RuleResultKey},
		{"RuleExecutionCauseKey", RuleExecutionCauseKey},
		{"ResourceKindKey", ResourceKindKey},
		{"ResourceNameKey", ResourceNameKey},
		{"ResourceNamespaceKey", ResourceNamespaceKey},
		{"ResourceVersionKey", ResourceVersionKey},
		{"ResourceOperationKey", ResourceOperationKey},
		{"RequestUserKey", RequestUserKey},
		{"RequestGroupsKey", RequestGroupsKey},
		{"CacheHitKey", CacheHitKey},
		{"CacheKeyKey", CacheKeyKey},
		{"QueueNameKey", QueueNameKey},
		{"QueueSizeKey", QueueSizeKey},
		{"EnginePhaseKey", EnginePhaseKey},
		{"EngineResultKey", EngineResultKey},
		{"PolicyTypeKey", PolicyTypeKey},
		{"PolicyValidationModeKey", PolicyValidationModeKey},
		{"PolicyBackgroundModeKey", PolicyBackgroundModeKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.key) == "" {
				t.Errorf("Expected %s to be non-empty", tt.name)
			}
		})
	}
}
