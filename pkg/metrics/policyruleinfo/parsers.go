package policyruleinfo

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/metrics"
)

func ParsePolicyRuleInfoMetricChangeType(change string) (PolicyRuleInfoMetricChangeType, error) {
	if change == "created" {
		return PolicyRuleCreated, nil
	}
	if change == "deleted" {
		return PolicyRuleDeleted, nil
	}
	return "", fmt.Errorf("wrong policy rule count metric change type found %s. Allowed: '%s', '%s'", change, "created", "deleted")
}

func ParsePromMetrics(pm metrics.PromMetrics) PromMetrics {
	return PromMetrics(pm)
}

func ParsePromConfig(pc metrics.PromConfig) PromConfig {
	return PromConfig(pc)
}
