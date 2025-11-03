package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apiserver/pkg/admission"
)

type metricWrapper struct {
	metrics            metrics.MutatingMetrics
	inner              Engine
	ruleExecutionCause string
}

func (w *metricWrapper) Evaluate(ctx context.Context, attr admission.Attributes, request admissionv1.AdmissionRequest, predicate Predicate) (EngineResponse, error) {
	response, err := w.inner.Evaluate(ctx, attr, request, predicate)
	if err != nil {
		return response, err
	}

	for _, policy := range response.Policies {
		if len(policy.Rules) == 0 {
			continue
		}

		w.metrics.RecordDuration(ctx, policy.Rules[0].Stats().ProcessingTime().Seconds(), string(policy.Rules[0].Status()), w.ruleExecutionCause, *policy.Policy, response.Resource, string(request.Operation))
	}

	return response, nil
}

func (w *metricWrapper) Handle(ctx context.Context, request engine.EngineRequest, predicate Predicate) (EngineResponse, error) {
	response, err := w.inner.Handle(ctx, request, predicate)
	if err != nil {
		return response, err
	}

	for _, policy := range response.Policies {
		if len(policy.Rules) == 0 {
			continue
		}

		w.metrics.RecordDuration(ctx, policy.Rules[0].Stats().ProcessingTime().Seconds(), string(policy.Rules[0].Status()), w.ruleExecutionCause, *policy.Policy, response.Resource, string(request.Request.Operation))
	}

	return response, nil
}

func (w *metricWrapper) MatchedMutateExistingPolicies(ctx context.Context, request engine.EngineRequest) []string {
	return w.inner.MatchedMutateExistingPolicies(ctx, request)
}

func NewMetricWrapper(inner Engine, ruleExecutionCause metrics.RuleExecutionCause) Engine {
	return &metricWrapper{
		inner:              inner,
		metrics:            metrics.GetMutatingMetrics(),
		ruleExecutionCause: string(ruleExecutionCause),
	}
}
