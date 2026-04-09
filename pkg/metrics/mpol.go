package metrics

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetMutatingMetrics() MutatingMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.MPOLMetrics()
}

type MutatingMetrics interface {
	RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1beta1.MutatingPolicyLike, resource *unstructured.Unstructured, operation string)
	RecordResult(ctx context.Context, status, ruleExecutionCause string, policy v1beta1.MutatingPolicyLike, resource *unstructured.Unstructured, operation string)
}

type mutatingMetrics struct {
	durationHistogram metric.Float64Histogram
	resultCounter     metric.Int64Counter

	logger logr.Logger
}

func (m *mutatingMetrics) init(meter metric.Meter) {
	var err error

	m.durationHistogram, err = meter.Float64Histogram(
		"kyverno_mutating_policy_execution_duration_seconds",
		metric.WithDescription("can be used to track the latencies (in seconds) associated with the execution/processing of individual mutating policies when they evaluate incoming resource requests."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_mutating_policy_execution_duration_seconds")
	}

	m.resultCounter, err = meter.Int64Counter(
		"kyverno_mutating_policy_results",
		metric.WithDescription("can be used to track the results associated with the mutating policies applied in the user's cluster, at the level from rule to policy to admission requests."),
	)
	if err != nil {
		m.logger.Error(err, "failed to register metric kyverno_mutating_policy_results")
	}
}

func (m *mutatingMetrics) RecordDuration(ctx context.Context, seconds float64, status, ruleExecutionCause string, policy v1beta1.MutatingPolicyLike, resource *unstructured.Unstructured, operation string) {
	if m.durationHistogram == nil {
		return
	}

	name, _, backgroundMode, _ := GetCELPolicyInfos(policy)

	m.durationHistogram.Record(ctx, seconds, metric.WithAttributes(
		attribute.String("policy_background_mode", string(backgroundMode)),
		attribute.String("policy_name", name),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("execution_cause", ruleExecutionCause),
		attribute.String("result", status),
	))
}

func (m *mutatingMetrics) RecordResult(ctx context.Context, status, ruleExecutionCause string, policy v1beta1.MutatingPolicyLike, resource *unstructured.Unstructured, operation string) {
	if m.resultCounter == nil {
		return
	}

	name, _, backgroundMode, _ := GetCELPolicyInfos(policy)

	m.resultCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("policy_background_mode", string(backgroundMode)),
		attribute.String("policy_name", name),
		attribute.String("resource_kind", resource.GetKind()),
		attribute.String("resource_namespace", resource.GetNamespace()),
		attribute.String("resource_request_operation", strings.ToLower(operation)),
		attribute.String("execution_cause", ruleExecutionCause),
		attribute.String("result", status),
	))
}
