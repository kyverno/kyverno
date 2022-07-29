package metrics

import (
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func NewFakeMetricsConfig(client kubernetes.Interface) *MetricsConfig {
	mc := &MetricsConfig{
		Config: config.NewFakeMetricsConfig(client),
		Log:    klog.NewKlogr(),
	}

	mc, _ = initializeMetrics(mc)
	return mc
}
