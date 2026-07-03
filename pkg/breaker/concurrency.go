package breaker

import (
	"context"

	"github.com/kyverno/kyverno/pkg/metrics"
)

// concurrencyLimiter decorates a Breaker with a bound on concurrent in-flight
// Do calls. When the bound is reached additional calls are dropped (returning
// nil, like an open breaker) and recorded through the breaker drop metrics
// under the limiter's own name. This bounds the damage when the inner call
// hangs (e.g. a reports API that accepts connections but never responds):
// report creations are fired at admission request rate, and without a bound
// the callers pile up until the pod is OOM killed.
type concurrencyLimiter struct {
	name  string
	inner Breaker
	slots chan struct{}
}

// WithConcurrencyLimit bounds the number of concurrent in-flight Do calls on
// the given breaker. A limit of zero or less returns the breaker unchanged.
func WithConcurrencyLimit(name string, limit int, inner Breaker) Breaker {
	if limit <= 0 {
		return inner
	}
	return &concurrencyLimiter{
		name:  name,
		inner: inner,
		slots: make(chan struct{}, limit),
	}
}

func (l *concurrencyLimiter) Do(ctx context.Context, inner func(context.Context) error) error {
	metrics := metrics.GetBreakerMetrics()

	if metrics != nil {
		metrics.RecordTotalIncrease(ctx, l.name)
	}
	select {
	case l.slots <- struct{}{}:
	default:
		if metrics != nil {
			metrics.RecordDrop(ctx, l.name)
		}
		return nil
	}
	defer func() {
		<-l.slots
	}()
	return l.inner.Do(ctx, inner)
}
