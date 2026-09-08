package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetLegacyPolicyMetrics() LegacyPolicyMetrics {
	if metricsConfig == nil {
		return nil
	}
	return metricsConfig.LegacyPolicyMetrics()
}

// LegacyPolicyMetrics observes, at scrape time, how many legacy (non
// policies.kyverno.io) policy custom resources currently exist in the
// cluster, per kind. Unlike kyverno_deprecated_api_requests_total (a counter
// of admission requests using deprecated APIs, i.e. flow), this is a gauge of
// stock: CRs that are stored and enforced today, whether or not any client is
// currently writing them.
type LegacyPolicyMetrics interface {
	// ObserveLegacyPolicyCount reports the number of existing legacy custom
	// resources for one kind.
	ObserveLegacyPolicyCount(ctx context.Context, observer metric.Observer, group, kind string, count int)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type legacyPolicyMetrics struct {
	legacyPoliciesMetric metric.Int64ObservableGauge
	meter                metric.Meter
	callback             metric.Callback

	logger logr.Logger
}

func (m *legacyPolicyMetrics) init(meter metric.Meter) {
	var err error

	m.legacyPoliciesMetric, err = meter.Int64ObservableGauge(
		"kyverno_legacy_policies_total",
		metric.WithDescription("can be used to track the number of legacy (non policies.kyverno.io) policy custom resources still present in the cluster, labeled by kind"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_legacy_policies_total")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.legacyPoliciesMetric); err != nil {
			m.logger.Error(err, "failed to register callback for legacy policies metric")
		}
	}
}

func (m *legacyPolicyMetrics) ObserveLegacyPolicyCount(ctx context.Context, observer metric.Observer, group, kind string, count int) {
	if m.legacyPoliciesMetric == nil {
		return
	}
	observer.ObserveInt64(m.legacyPoliciesMetric, int64(count), metric.WithAttributes(
		attribute.String("group", group),
		attribute.String("kind", kind),
	))
}

func (m *legacyPolicyMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.legacyPoliciesMetric)
}
