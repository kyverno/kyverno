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

// GlobalContextMetrics tracks how GlobalContextEntry projections are being migrated from
// JMESPath to CEL. Both counters are deliberately bounded: language has exactly two values
// (jmespath, cel), and function is restricted to Kyverno's known JMESPath function registry
// (see pkg/engine/jmespath.UsedFunctionNames). Neither is ever labeled with expression text,
// projection name, GlobalContextEntry name, or namespace - any of those would make the metric's
// cardinality unbounded.
type GlobalContextMetrics interface {
	// RecordProjectionLanguage records one projection using the given language (jmespath or cel).
	RecordProjectionLanguage(ctx context.Context, language string)
	// RecordJMESPathFunctionUsage records one projection's use of a known JMESPath function.
	RecordJMESPathFunctionUsage(ctx context.Context, function string)
}

type globalContextMetrics struct {
	languageMetric metric.Int64Counter
	functionMetric metric.Int64Counter

	logger logr.Logger
}

func (m *globalContextMetrics) init(meter metric.Meter) {
	var err error

	m.languageMetric, err = meter.Int64Counter(
		"kyverno_globalcontextentry_projection_language",
		metric.WithDescription("can be used to track GlobalContextEntry projections by language, as Kyverno migrates projections from JMESPath to CEL"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_globalcontextentry_projection_language")
	}

	m.functionMetric, err = meter.Int64Counter(
		"kyverno_globalcontextentry_jmespath_function_usage",
		metric.WithDescription("can be used to track which JMESPath functions GlobalContextEntry projections still use, to prioritize CEL migration support"),
	)
	if err != nil {
		m.logger.Error(err, "Failed to create instrument, kyverno_globalcontextentry_jmespath_function_usage")
	}
}

func (m *globalContextMetrics) RecordProjectionLanguage(ctx context.Context, language string) {
	if m.languageMetric == nil {
		return
	}
	m.languageMetric.Add(ctx, 1, metric.WithAttributes(attribute.String("language", language)))
}

func (m *globalContextMetrics) RecordJMESPathFunctionUsage(ctx context.Context, function string) {
	if m.functionMetric == nil {
		return
	}
	m.functionMetric.Add(ctx, 1, metric.WithAttributes(attribute.String("function", function)))
}
