package metrics

import (
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/client-go/kubernetes"
)

func NewFakeMetricsConfig(client kubernetes.Interface) *config.MetricsConfigData {
	metricsConfig := config.NewFakeMetricsConfig(client)
	return metricsConfig
}

func NewFakePromConfig(client kubernetes.Interface) (*PromConfig, error) {
	metricsConfig := config.NewFakeMetricsConfig(client)
	return NewPromConfig(metricsConfig)
}
