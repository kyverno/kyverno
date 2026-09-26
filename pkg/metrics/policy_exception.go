package metrics

import (
	"context"

	"github.com/go-logr/logr"
	kconfig "github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetPolicyExceptionMetrics() PolicyExceptionMetrics {
	if metricsConfig == nil {
		return nil
	}

	return metricsConfig.PolicyExceptionMetrics()
}

// PolicyExceptionMetrics reports the current PolicyException inventory. Each
// observation identifies one exception and one policy reference; the value is
// always one because this is an inventory gauge rather than a counter.
type PolicyExceptionMetrics interface {
	RecordPolicyExceptionInfo(ctx context.Context, observer metric.Observer, apiGroup, exceptionNamespace, exceptionName, policyKind, policyName string)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type policyExceptionMetrics struct {
	infoMetric metric.Int64ObservableGauge
	meter      metric.Meter
	callback   metric.Callback
	config     kconfig.MetricsConfiguration

	logger logr.Logger
}

func (m *policyExceptionMetrics) init(meter metric.Meter) {
	var err error

	m.infoMetric, err = meter.Int64ObservableGauge(
		"kyverno_policy_exception_info",
		metric.WithDescription("current PolicyException resources and their declared policy references; value is always 1"),
	)
	if err != nil {
		m.logger.Error(err, "failed to create instrument, kyverno_policy_exception_info")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.infoMetric); err != nil {
			m.logger.Error(err, "failed to register callback for policy exception info metric")
		}
	}
}

func (m *policyExceptionMetrics) RecordPolicyExceptionInfo(ctx context.Context, observer metric.Observer, apiGroup, exceptionNamespace, exceptionName, policyKind, policyName string) {
	if m.infoMetric == nil {
		return
	}
	if m.config != nil && !m.config.CheckNamespace(exceptionNamespace) {
		return
	}

	observer.ObserveInt64(m.infoMetric, 1, metric.WithAttributes(
		attribute.String("exception_api_group", apiGroup),
		attribute.String("exception_namespace", exceptionNamespace),
		attribute.String("exception_name", exceptionName),
		attribute.String("policy_kind", policyKind),
		attribute.String("policy_name", policyName),
	))
}

func (m *policyExceptionMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	m.callback = f
	if m.meter == nil {
		return nil, nil
	}

	return m.meter.RegisterCallback(f, m.infoMetric)
}
