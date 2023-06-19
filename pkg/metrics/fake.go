package metrics

import (
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel"
	"k8s.io/klog/v2"
)

func NewFakeMetricsConfig() *MetricsConfig {
	mc := &MetricsConfig{
		config: config.NewDefaultMetricsConfiguration(),
		Log:    klog.NewKlogr(),
	}
	_ = mc.initializeMetrics(otel.GetMeterProvider())
	return mc
}
