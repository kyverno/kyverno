package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResourceMetrics(t *testing.T) {
	logger := logr.Discard()

	rm, err := NewResourceMetrics(logger)
	require.NoError(t, err)
	require.NotNil(t, rm)

	assert.Equal(t, logger, rm.logger)
	assert.NotNil(t, rm.meter)
	assert.NotNil(t, rm.memoryUsageGauge)
	assert.NotNil(t, rm.goroutineCountGauge)
	assert.NotNil(t, rm.gcCountCounter)
}

func TestResourceMetrics_RecordCacheHit(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Should not panic and should record metric
	rm.RecordCacheHit(ctx, "policy")
	rm.RecordCacheHit(ctx, "resource")
}

func TestResourceMetrics_RecordCacheMiss(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Should not panic and should record metric
	rm.RecordCacheMiss(ctx, "policy")
	rm.RecordCacheMiss(ctx, "resource")
}

func TestResourceMetrics_RecordProcessingTime(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Test different operations and durations
	testCases := []struct {
		operation string
		duration  time.Duration
	}{
		{"validation", 100 * time.Millisecond},
		{"mutation", 50 * time.Millisecond},
		{"generation", 200 * time.Millisecond},
		{"policy_processing", 500 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(tc.operation, func(t *testing.T) {
			// Should not panic and should record metric
			rm.RecordProcessingTime(ctx, tc.operation, tc.duration)
		})
	}
}

func TestResourceMetrics_RecordAdmissionLatency(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Test different resources and latencies
	testCases := []struct {
		resource string
		latency  time.Duration
	}{
		{"Pod", 10 * time.Millisecond},
		{"Deployment", 25 * time.Millisecond},
		{"Service", 5 * time.Millisecond},
		{"ConfigMap", 15 * time.Millisecond},
	}

	for _, tc := range testCases {
		t.Run(tc.resource, func(t *testing.T) {
			// Should not panic and should record metric
			rm.RecordAdmissionLatency(ctx, tc.latency, tc.resource)
		})
	}
}

func TestResourceMetrics_RecordGCCycle(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Should not panic and should record metric
	rm.RecordGCCycle(ctx)
	rm.RecordGCCycle(ctx)
	rm.RecordGCCycle(ctx)
}

func TestResourceMetrics_CacheOperations(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Test cache hit/miss patterns
	cacheTypes := []string{"policy", "resource", "binding"}

	for _, cacheType := range cacheTypes {
		// Record some cache hits
		for i := 0; i < 5; i++ {
			rm.RecordCacheHit(ctx, cacheType)
		}

		// Record some cache misses
		for i := 0; i < 2; i++ {
			rm.RecordCacheMiss(ctx, cacheType)
		}
	}
}

func TestResourceMetrics_ProcessingTimePatterns(t *testing.T) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(t, err)

	ctx := context.Background()

	// Simulate various processing time patterns
	operations := []string{"validation", "mutation", "generation"}
	durations := []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		250 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, op := range operations {
		for _, duration := range durations {
			rm.RecordProcessingTime(ctx, op, duration)
		}
	}
}

func BenchmarkResourceMetrics_RecordCacheHit(b *testing.B) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordCacheHit(ctx, "policy")
	}
}

func BenchmarkResourceMetrics_RecordProcessingTime(b *testing.B) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()
	duration := 100 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordProcessingTime(ctx, "validation", duration)
	}
}

func BenchmarkResourceMetrics_RecordAdmissionLatency(b *testing.B) {
	rm, err := NewResourceMetrics(logr.Discard())
	require.NoError(b, err)

	ctx := context.Background()
	latency := 25 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.RecordAdmissionLatency(ctx, latency, "Pod")
	}
}
