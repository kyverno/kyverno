package engine

import (
	"context"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/metrics"
)

type metricWrapper struct {
	metrics            metrics.ValidatingMetrics
	inner              Engine
	ruleExecutionCause string
}

func (w *metricWrapper) Handle(ctx context.Context, request EngineRequest, predicate func(v1alpha1.ValidatingPolicy) bool) (EngineResponse, error) {
	response, err := w.inner.Handle(ctx, request, predicate)
	if err != nil {
		return response, err
	}

	for _, policy := range response.Policies {
		if len(policy.Rules) == 0 {
			continue
		}

		w.metrics.RecordDuration(ctx, policy.Rules[0].Stats().ProcessingTime().Seconds(), string(policy.Rules[0].Status()), w.ruleExecutionCause, policy.Policy, response.Resource, string(request.Request.Operation))
	}

	return response, nil
}

func NewMetricWrapper(inner Engine, ruleExecutionCause metrics.RuleExecutionCause) Engine {
	return &metricWrapper{
		inner:              inner,
		metrics:            metrics.GetValidatingMetrics(),
		ruleExecutionCause: string(ruleExecutionCause),
	}
}
