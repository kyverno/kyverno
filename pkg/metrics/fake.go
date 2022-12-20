package metrics

import (
	"github.com/kyverno/kyverno/pkg/config"
	"go.opentelemetry.io/otel/metric/global"
	"k8s.io/klog/v2"
)

func NewFakeMetricsConfig() *MetricsConfig {
	mc := &MetricsConfig{
		config: config.NewDefaultMetricsConfiguration(),
		Log:    klog.NewKlogr(),
	}
	_ = mc.initializeMetrics(global.MeterProvider())
	return mc
}
