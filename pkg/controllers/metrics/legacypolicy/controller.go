package legacypolicy

import (
	"context"

	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel/metric"
)

// controller reports the kyverno_legacy_policies_total gauge at every scrape,
// by recomputing counts from the counters it was given. It is shared by the
// admission controller and the cleanup controller: each process constructs it
// with counters for only the legacy kinds it natively watches through synced
// listers.
type controller struct {
	metrics  metrics.LegacyPolicyMetrics
	group    string
	counters map[string]deprecations.KindCounter
}

// NewController registers a scrape-time callback for the kyverno_legacy_policies_total
// gauge, backed by counters. Counters is typically built from synced informer
// listers, one KindCounter per legacy kind this process natively watches.
//
// TODO: like policy-metrics, this only registers a callback, it should be changed
// to a real controller.
func NewController(m metrics.LegacyPolicyMetrics, group string, counters map[string]deprecations.KindCounter) {
	c := &controller{
		metrics:  m,
		group:    group,
		counters: counters,
	}
	if c.metrics == nil {
		return
	}
	if _, err := c.metrics.RegisterCallback(c.report); err != nil {
		logger.Error(err, "failed to register callback for legacy policies metric")
	}
}

func (c *controller) report(ctx context.Context, observer metric.Observer) error {
	// CountLegacyPolicies omits any kind whose counter failed (it does not
	// report that kind as zero) and returns the successful counts alongside the
	// error. Each kind is an independent gauge series, so observe whatever was
	// collected and return nil: a single failing lister then drops only its own
	// kind's series rather than making the whole metric disappear from the scrape.
	counts, err := deprecations.CountLegacyPolicies(c.counters)
	if err != nil {
		logger.Error(err, "failed to count some legacy policies; exporting the counts that succeeded")
	}
	for kind, count := range counts {
		c.metrics.ObserveLegacyPolicyCount(ctx, observer, c.group, kind, count)
	}
	return nil
}
