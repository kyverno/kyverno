package policychanges

import (
	"github.com/kyverno/kyverno/pkg/metrics"
)

func ParsePromMetrics(pm metrics.PromMetrics) PromMetrics {
	return PromMetrics(pm)
}

func ParsePromConfig(pc metrics.PromConfig) PromConfig {
	return PromConfig(pc)
}
