package metrics

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestGlobalContextMetrics_RecordProjectionLanguage(t *testing.T) {
	counter := &fakeCounter{}
	m := &globalContextMetrics{languageMetric: counter, logger: logr.Discard()}

	m.RecordProjectionLanguage(context.Background(), "jmespath")

	assert.Equal(t, 1, counter.calls)
	assert.Contains(t, counter.attributes, attribute.String("language", "jmespath"))
}

func TestGlobalContextMetrics_RecordJMESPathFunctionUsage(t *testing.T) {
	counter := &fakeCounter{}
	m := &globalContextMetrics{functionMetric: counter, logger: logr.Discard()}

	m.RecordJMESPathFunctionUsage(context.Background(), "add")

	assert.Equal(t, 1, counter.calls)
	assert.Contains(t, counter.attributes, attribute.String("function", "add"))
}

func TestGlobalContextMetrics_NilInstrumentGuard(t *testing.T) {
	m := &globalContextMetrics{logger: logr.Discard()}

	// Neither instrument was created (e.g. meter.Int64Counter failed); recording must not panic.
	assert.NotPanics(t, func() {
		m.RecordProjectionLanguage(context.Background(), "jmespath")
		m.RecordJMESPathFunctionUsage(context.Background(), "add")
	})
}
