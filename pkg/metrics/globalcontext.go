package metrics

import (
	"context"

	"github.com/go-logr/logr"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func GetGlobalContextMetrics() GlobalContextMetrics {
	if metricsConfig == nil {
		return nil
	}
	return metricsConfig.GlobalContextMetrics()
}

// GlobalContextMetrics tracks how GlobalContextEntry projections are configured, to help
// migrate them from JMESPath to CEL. These are gauges of current spec state (how many
// projections, right now, use each language/function), not counters of events: the
// globalcontext controller is not leader-elected and reconciles on every replica, so a
// counter incremented per reconcile would multiply by replica count. A gauge recomputed
// from the current GlobalContextEntry list is idempotent per replica - every replica
// reports the same value, so scraping all of them and taking the max (not the sum) gives
// the correct cluster-wide answer. Both instruments are bounded cardinality: language has
// exactly two values (jmespath, cel), and function is restricted to Kyverno's known
// JMESPath function registry (see pkg/engine/jmespath.UsedFunctionNames). Neither is ever
// labeled with expression text, projection name, GlobalContextEntry name, or namespace -
// any of those would make the metric's cardinality unbounded.
type GlobalContextMetrics interface {
	// ObserveProjectionLanguage reports the number of projections, across every current
	// GlobalContextEntry, written in the given language (jmespath or cel).
	ObserveProjectionLanguage(ctx context.Context, observer metric.Observer, language string, count int)
	// ObserveJMESPathFunctionUsage reports the number of projections, across every current
	// GlobalContextEntry, that call the given known JMESPath function.
	ObserveJMESPathFunctionUsage(ctx context.Context, observer metric.Observer, function string, count int)
	RegisterCallback(f metric.Callback) (metric.Registration, error)
}

type globalContextMetrics struct {
	languageMetric metric.Int64ObservableGauge
	functionMetric metric.Int64ObservableGauge
	meter          metric.Meter
	callback       metric.Callback

	logger logr.Logger
}

func (m *globalContextMetrics) init(meter metric.Meter) {
	var err error

	m.languageMetric, err = meter.Int64ObservableGauge(
		"kyverno_globalcontextentry_projection_language",
		metric.WithDescription("the current number of GlobalContextEntry projections by language, as Kyverno migrates projections from JMESPath to CEL; reported identically by every replica, take the max across instances rather than summing"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_globalcontextentry_projection_language")
	}

	m.functionMetric, err = meter.Int64ObservableGauge(
		"kyverno_globalcontextentry_jmespath_function_usage",
		metric.WithDescription("the current number of GlobalContextEntry projections still calling each known JMESPath function, to prioritize CEL migration support; reported identically by every replica, take the max across instances rather than summing"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_globalcontextentry_jmespath_function_usage")
	}

	m.meter = meter

	if m.callback != nil {
		if _, err := m.meter.RegisterCallback(m.callback, m.languageMetric, m.functionMetric); err != nil {
			m.logger.Error(err, "failed to register callback for global context metrics")
		}
	}
}

func (m *globalContextMetrics) ObserveProjectionLanguage(ctx context.Context, observer metric.Observer, language string, count int) {
	if m.languageMetric == nil {
		return
	}
	observer.ObserveInt64(m.languageMetric, int64(count), metric.WithAttributes(attribute.String("language", language)))
}

func (m *globalContextMetrics) ObserveJMESPathFunctionUsage(ctx context.Context, observer metric.Observer, function string, count int) {
	if m.functionMetric == nil {
		return
	}
	observer.ObserveInt64(m.functionMetric, int64(count), metric.WithAttributes(attribute.String("function", function)))
}

func (m *globalContextMetrics) RegisterCallback(f metric.Callback) (metric.Registration, error) {
	if m.meter == nil {
		return nil, nil
	}

	m.callback = f
	return m.meter.RegisterCallback(f, m.languageMetric, m.functionMetric)
}
