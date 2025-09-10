package engine

import (
	"context"

	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	"github.com/kyverno/kyverno/pkg/metrics"
	"gomodules.xyz/jsonpatch/v2"
)

type metricWrapper struct {
	metrics            metrics.ImageValidatingMetrics
	inner              Engine
	ruleExecutionCause string
}

func (w *metricWrapper) HandleMutating(ctx context.Context, request EngineRequest, predicate Predicate) (eval.ImageVerifyEngineResponse, []jsonpatch.JsonPatchOperation, error) {
	response, patch, err := w.inner.HandleMutating(ctx, request, predicate)
	if err != nil {
		return response, nil, err
	}

	for _, policy := range response.Policies {
		w.metrics.RecordDuration(ctx, policy.Result.Stats().ProcessingTime().Seconds(), string(policy.Result.Status()), w.ruleExecutionCause, policy.Policy, response.Resource, string(request.Request.Operation))
	}

	return response, patch, nil
}

func (w *metricWrapper) HandleValidating(ctx context.Context, request EngineRequest, predicate Predicate) (eval.ImageVerifyEngineResponse, error) {
	response, err := w.inner.HandleValidating(ctx, request, predicate)
	if err != nil {
		return response, err
	}

	for _, policy := range response.Policies {
		w.metrics.RecordDuration(ctx, policy.Result.Stats().ProcessingTime().Seconds(), string(policy.Result.Status()), w.ruleExecutionCause, policy.Policy, response.Resource, string(request.Request.Operation))
	}

	return response, nil
}

func NewMetricWrapper(inner Engine, ruleExecutionCause metrics.RuleExecutionCause) Engine {
	return &metricWrapper{
		inner:              inner,
		metrics:            metrics.GetImageValidatingMetrics(),
		ruleExecutionCause: string(ruleExecutionCause),
	}
}
