package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPolicyMetrics(t *testing.T) {
	logger := logr.Discard()
	pm, err := NewPolicyMetrics(logger)
	require.NoError(t, err)
	assert.NotNil(t, pm)
	assert.NotNil(t, pm.logger)
	assert.NotNil(t, pm.meter)
	assert.NotNil(t, pm.policyProcessingDuration)
	assert.NotNil(t, pm.ruleEvaluationDuration)
	assert.NotNil(t, pm.policyEvaluationCounter)
	assert.NotNil(t, pm.policyCacheHitCounter)
	assert.NotNil(t, pm.policyCacheMissCounter)
	assert.NotNil(t, pm.admissionRequestDuration)
	assert.NotNil(t, pm.admissionRequestCounter)
}

func TestPolicyMetrics_RecordPolicyCacheHit(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Should not panic
	pm.RecordPolicyCacheHit(ctx, "policy")
	pm.RecordPolicyCacheHit(ctx, "resource")
}

func TestPolicyMetrics_RecordPolicyCacheMiss(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Should not panic
	pm.RecordPolicyCacheMiss(ctx, "policy")
	pm.RecordPolicyCacheMiss(ctx, "resource")
}

func TestPolicyMetrics_RecordPolicyProcessingDuration(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()
	duration := 100 * time.Millisecond

	testCases := []struct {
		policyType string
		ruleType   RuleType
		result     RuleResult
	}{
		{"ClusterPolicy", Validate, Pass},
		{"Policy", Mutate, Fail},
		{"ClusterPolicy", Generate, Pass},
		{"Policy", ImageVerify, Error},
	}

	for _, tc := range testCases {
		t.Run(string(tc.ruleType)+"_"+string(tc.result), func(t *testing.T) {
			// Should not panic
			pm.RecordPolicyProcessingDuration(ctx, duration, tc.policyType, tc.ruleType, tc.result)
		})
	}
}

func TestPolicyMetrics_RecordRuleEvaluationDuration(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()
	duration := 50 * time.Millisecond

	// Should not panic
	pm.RecordRuleEvaluationDuration(ctx, duration, "test-policy", "test-rule", Validate)
	pm.RecordRuleEvaluationDuration(ctx, duration, "test-policy-2", "test-rule-2", Mutate)
}

func TestPolicyMetrics_RecordPolicyEvaluation(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	testCases := []struct {
		policyType string
		ruleType   RuleType
		result     RuleResult
		cause      RuleExecutionCause
	}{
		{"ClusterPolicy", Validate, Pass, AdmissionRequest},
		{"Policy", Mutate, Fail, BackgroundScan},
		{"ClusterPolicy", Generate, Pass, AdmissionRequest},
	}

	for _, tc := range testCases {
		t.Run(string(tc.ruleType)+"_"+string(tc.result)+"_"+string(tc.cause), func(t *testing.T) {
			// Should not panic
			pm.RecordPolicyEvaluation(ctx, tc.policyType, tc.ruleType, tc.result, tc.cause)
		})
	}
}

func TestPolicyMetrics_RecordAdmissionRequestDuration(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()
	duration := 200 * time.Millisecond

	testCases := []struct {
		operation    ResourceRequestOperation
		resourceKind string
		allowed      bool
	}{
		{ResourceCreated, "Pod", true},
		{ResourceUpdated, "Service", false},
		{ResourceDeleted, "ConfigMap", true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.operation)+"_"+tc.resourceKind, func(t *testing.T) {
			// Should not panic
			pm.RecordAdmissionRequestDuration(ctx, duration, tc.operation, tc.resourceKind, tc.allowed)
		})
	}
}

func TestPolicyMetrics_RecordAdmissionRequest(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	testCases := []struct {
		operation    ResourceRequestOperation
		resourceKind string
		allowed      bool
	}{
		{ResourceCreated, "Pod", true},
		{ResourceUpdated, "Service", false},
		{ResourceDeleted, "ConfigMap", true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.operation)+"_"+tc.resourceKind, func(t *testing.T) {
			// Should not panic
			pm.RecordAdmissionRequest(ctx, tc.operation, tc.resourceKind, tc.allowed)
		})
	}
}

func TestPolicyMetrics_SetPolicyCacheSize(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	// Should not panic
	pm.SetPolicyCacheSize(42)
	pm.SetPolicyCacheSize(0)
	pm.SetPolicyCacheSize(1000)
}

func TestPolicyMetrics_CacheOperations(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Test recording multiple cache operations
	for i := 0; i < 10; i++ {
		pm.RecordPolicyCacheHit(ctx, "policy")
	}

	for i := 0; i < 3; i++ {
		pm.RecordPolicyCacheMiss(ctx, "policy")
	}

	// Should not panic
	assert.NotPanics(t, func() {
		pm.SetPolicyCacheSize(100)
	})
}

func TestPolicyMetrics_PolicyProcessingPatterns(t *testing.T) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Test different processing patterns
	durations := []time.Duration{
		10 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
	}

	for _, duration := range durations {
		pm.RecordPolicyProcessingDuration(ctx, duration, "ClusterPolicy", Validate, Pass)
		pm.RecordRuleEvaluationDuration(ctx, duration/2, "test-policy", "test-rule", Validate)
	}

	// Test admission request patterns
	for _, duration := range durations {
		pm.RecordAdmissionRequestDuration(ctx, duration, ResourceCreated, "Pod", true)
		pm.RecordAdmissionRequest(ctx, ResourceCreated, "Pod", true)
	}
}

func BenchmarkPolicyMetrics_RecordPolicyCacheHit(b *testing.B) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordPolicyCacheHit(ctx, "policy")
	}
}

func BenchmarkPolicyMetrics_RecordPolicyProcessingDuration(b *testing.B) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()
	duration := 100 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordPolicyProcessingDuration(ctx, duration, "ClusterPolicy", Validate, Pass)
	}
}

func BenchmarkPolicyMetrics_RecordAdmissionRequestDuration(b *testing.B) {
	pm, err := NewPolicyMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()
	duration := 200 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.RecordAdmissionRequestDuration(ctx, duration, ResourceCreated, "Pod", true)
	}
}
