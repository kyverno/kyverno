package policyresults

import (
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func ParsePromMetrics(pm metrics.PromMetrics) PromMetrics {
	return PromMetrics(pm)
}

func ParsePromConfig(pc metrics.PromConfig) PromConfig {
	return PromConfig(pc)
}

func ParseRuleTypeFromEngineRuleResponse(rule response.RuleResponse) metrics.RuleType {
	switch rule.Type {
	case "Validation":
		return metrics.Validate
	case "Mutation":
		return metrics.Mutate
	case "Generation":
		return metrics.Generate
	default:
		return metrics.EmptyRuleType
	}
}
