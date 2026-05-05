package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/metrics"
)

type metricWrapper struct {
	metrics metrics.GeneratingMetrics
	inner   Engine
}

func NewMetricsEngine(engine Engine) *metricWrapper {
	return &metricWrapper{
		metrics: metrics.GetGeneratingMetrics(),
		inner:   engine,
	}
}

func (w *metricWrapper) Handle(request engine.EngineRequest, policy Policy, cacheRestore bool) (EngineResponse, error) {
	response, err := w.inner.Handle(request, policy, cacheRestore)
	if err != nil {
		return response, err
	}

	for _, policy := range response.Policies {
		if policy.Result == nil {
			continue
		}

		w.metrics.RecordDuration(context.TODO(), policy.Result.Stats().ProcessingTime().Seconds(), string(policy.Result.Status()), policy.Policy, response.Trigger, string(request.Request.Operation))
	}

	return response, nil
}
