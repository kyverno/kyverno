package legacypolicy

import (
	"context"
	"errors"
	"testing"

	"github.com/kyverno/kyverno/pkg/deprecations"
	"go.opentelemetry.io/otel/metric"
)

type observation struct {
	group string
	kind  string
	count int
}

// stubMetrics records ObserveLegacyPolicyCount calls; a nil observer is fine since report
// only reads from the cache under test, never from the opentelemetry callback machinery.
type stubMetrics struct {
	observed []observation
}

func (s *stubMetrics) ObserveLegacyPolicyCount(_ context.Context, _ metric.Observer, group, kind string, count int) {
	s.observed = append(s.observed, observation{group: group, kind: kind, count: count})
}

func (s *stubMetrics) RegisterCallback(metric.Callback) (metric.Registration, error) {
	return nil, nil
}

func TestController_Refresh(t *testing.T) {
	c := &controller{
		counters: map[string]deprecations.KindCounter{
			"ClusterPolicy": func(context.Context) (int, error) { return 2, nil },
			"Policy":        func(context.Context) (int, error) { return 3, nil },
		},
	}
	c.refresh(context.Background())

	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.counts) != 2 {
		t.Fatalf("counts = %v, want 2 entries", c.counts)
	}
	if c.counts["ClusterPolicy"] != 2 || c.counts["Policy"] != 3 {
		t.Errorf("counts = %v, want ClusterPolicy=2, Policy=3", c.counts)
	}
}

func TestController_Refresh_FailingCounterDropsOnlyItsKind(t *testing.T) {
	c := &controller{
		counters: map[string]deprecations.KindCounter{
			"ClusterPolicy": func(context.Context) (int, error) { return 5, nil },
			"Policy":        func(context.Context) (int, error) { return 0, errors.New("list failed") },
		},
	}
	c.refresh(context.Background())

	c.mu.RLock()
	defer c.mu.RUnlock()
	if _, ok := c.counts["Policy"]; ok {
		t.Errorf("counts = %v, want no Policy entry after a failing counter", c.counts)
	}
	if c.counts["ClusterPolicy"] != 5 {
		t.Errorf("counts[ClusterPolicy] = %d, want 5", c.counts["ClusterPolicy"])
	}
}

func TestController_Report(t *testing.T) {
	m := &stubMetrics{}
	c := &controller{
		metrics: m,
		group:   "kyverno.io",
		counts:  map[string]int{"ClusterPolicy": 7},
	}
	if err := c.report(context.Background(), nil); err != nil {
		t.Fatalf("report() unexpected error: %v", err)
	}
	if len(m.observed) != 1 || m.observed[0] != (observation{group: "kyverno.io", kind: "ClusterPolicy", count: 7}) {
		t.Errorf("observed = %v, want one ClusterPolicy=7 observation", m.observed)
	}
}
