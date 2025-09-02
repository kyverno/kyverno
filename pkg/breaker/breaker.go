package breaker

import (
	"context"

	"github.com/kyverno/kyverno/pkg/metrics"
)

var ReportsBreaker Breaker

func GetReportsBreaker() Breaker { return ReportsBreaker }

type Breaker interface {
	Do(context.Context, func(context.Context) error) error
}

type breaker struct {
	name    string
	metrics metrics.BreakerMetrics
	open    func(context.Context) bool
}

func NewBreaker(name string, metrics metrics.BreakerMetrics, open func(context.Context) bool) *breaker {
	return &breaker{
		name:    name,
		metrics: metrics,
		open:    open,
	}
}

func (b *breaker) Do(ctx context.Context, inner func(context.Context) error) error {
	if b.metrics != nil {
		b.metrics.RecordTotalIncrease(ctx, b.name)
	}
	if b.open != nil && b.open(ctx) {
		if b.metrics != nil {
			b.metrics.RecordDrop(ctx, b.name)
		}
		return nil
	}
	if inner == nil {
		return nil
	}
	return inner(ctx)
}
