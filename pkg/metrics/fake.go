package metrics

import (
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/klog/v2"
)

func NewFakeMetricsConfig() *MetricsConfig {
	mc := &MetricsConfig{
		Config: config.NewDefaultMetricsConfiguration(),
		Log:    klog.NewKlogr(),
	}
	_ = mc.initializeMetrics()
	return mc
}
