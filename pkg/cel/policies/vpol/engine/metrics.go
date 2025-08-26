package engine

import (
	"context"
	"strconv"
	"strings"

	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var durationHistogram metric.Float64Histogram

type metricWrapper struct {
	durationHistogram  metric.Float64Histogram
	inner              Engine
	ruleExecutionCause metrics.RuleExecutionCause
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

		name, _, backgroundMode, validationMode := metrics.GetCELPolicyInfos(&policy.Policy)

		w.durationHistogram.Record(ctx, policy.Rules[0].Stats().ProcessingTime().Seconds(), metric.WithAttributes(
			attribute.String("policy_validation_mode", string(validationMode)),
			attribute.String("policy_background_mode", string(backgroundMode)),
			attribute.String("policy_name", name),
			attribute.String("resource_kind", response.Resource.GetKind()),
			attribute.String("resource_namespace", response.Resource.GetNamespace()),
			attribute.String("resource_request_operation", strings.ToLower(string(request.Request.Operation))),
			attribute.String("rule_execution_cause", string(w.ruleExecutionCause)),
			attribute.String("result", string(policy.Rules[0].Status())),
			attribute.String("dry_run", strconv.FormatBool(*request.Request.DryRun)),
		))
	}

	return response, nil
}

func NewMetricWrapper(inner Engine, ruleExecutionCause metrics.RuleExecutionCause) (Engine, error) {
	if durationHistogram == nil {
		var err error

		meter := otel.GetMeterProvider().Meter(metrics.MeterName)

		durationHistogram, err = meter.Float64Histogram(
			"kyverno_validating_policy_execution_duration_seconds",
			metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of individual validation policies when they evaluate incoming resource requests."),
		)

		if err != nil {
			logging.Error(err, "failed to register metric kyverno_validating_policy_execution_duration_seconds")
			return nil, err
		}
	}

	return &metricWrapper{
		inner:              inner,
		durationHistogram:  durationHistogram,
		ruleExecutionCause: ruleExecutionCause,
	}, nil
}
