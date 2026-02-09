package breaker

import (
	"context"
	"sync/atomic"

	"github.com/kyverno/kyverno/pkg/metrics"
)

var reportsBreaker atomic.Value

func GetReportsBreaker() Breaker {
	if v := reportsBreaker.Load(); v != nil {
		return v.(Breaker)
	}
	return nil
}

func SetReportsBreaker(b Breaker) {
	reportsBreaker.Store(b)
}

type Breaker interface {
	Do(context.Context, func(context.Context) error) error
}

type breaker struct {
	name string
	open func(context.Context) bool
}

func NewBreaker(name string, open func(context.Context) bool) *breaker {
	return &breaker{
		name: name,
		open: open,
	}
}

func (b *breaker) Do(ctx context.Context, inner func(context.Context) error) error {
	metrics := metrics.GetBreakerMetrics()

	if metrics != nil {
		metrics.RecordTotalIncrease(ctx, b.name)
	}
	if b.open != nil && b.open(ctx) {
		if metrics != nil {
			metrics.RecordDrop(ctx, b.name)
		}
		return nil
	}
	if inner == nil {
		return nil
	}
	return inner(ctx)
}
