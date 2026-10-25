package legacypolicy

import (
	"context"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	ControllerName  = "legacy-policy-metrics"
	Workers         = 1
	refreshInterval = 30 * time.Second
	refreshTimeout  = 10 * time.Second
)

// controller reports the kyverno_legacy_policies_total gauge at every scrape, from counts
// refreshed in the background every refreshInterval. It is shared by the admission
// controller and the cleanup controller: each process constructs it with counters for only
// the legacy kinds it natively watches.
type controller struct {
	metrics  metrics.LegacyPolicyMetrics
	group    string
	counters map[string]deprecations.KindCounter

	mu     sync.RWMutex
	counts map[string]int
}

// NewController registers the scrape callback immediately, as today, so the gauge exists from
// the first scrape; Run only drives the refresh loop.
func NewController(m metrics.LegacyPolicyMetrics, group string, counters map[string]deprecations.KindCounter) controllers.Controller {
	c := &controller{
		metrics:  m,
		group:    group,
		counters: counters,
	}
	if c.metrics != nil {
		if _, err := c.metrics.RegisterCallback(c.report); err != nil {
			logger.Error(err, "failed to register callback for legacy policies metric")
		}
	}
	return c
}

// Run refreshes the counts every refreshInterval until ctx is done. The caller only
// constructs this controller when metrics export is enabled, so Run always has somewhere
// to send what it refreshes.
func (c *controller) Run(ctx context.Context, _ int) {
	wait.UntilWithContext(ctx, c.refresh, refreshInterval)
}

// refresh replaces the cached counts; a kind whose counter failed is dropped until the next
// success, exactly as today's per-scrape behaviour.
func (c *controller) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	counts, err := deprecations.CountLegacyPolicies(ctx, c.counters)
	if err != nil {
		logger.Error(err, "failed to count some legacy policies; exporting the counts that succeeded")
	}
	c.mu.Lock()
	c.counts = counts
	c.mu.Unlock()
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for kind, count := range c.counts {
		c.metrics.ObserveLegacyPolicyCount(ctx, observer, c.group, kind, count)
	}
	return nil
}
