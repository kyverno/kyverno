package utils

import "github.com/kyverno/kyverno/pkg/metrics"

type ClientQueryMetric interface {
	Record(clientQueryOperation metrics.ClientQueryOperation, clientType metrics.ClientType, resourceKind string, resourceNamespace string)
}

type metricsConfig struct {
	metricsConfig *metrics.MetricsConfig
}

func NewClientQueryMetric(m *metrics.MetricsConfig) ClientQueryMetric {
	return &metricsConfig{
		metricsConfig: m,
	}
}

func (c *metricsConfig) Record(clientQueryOperation metrics.ClientQueryOperation, clientType metrics.ClientType, resourceKind string, resourceNamespace string) {
	if c.metricsConfig == nil {
		return
	}
	c.metricsConfig.RecordClientQueries(clientQueryOperation, clientType, resourceKind, resourceNamespace)
}
