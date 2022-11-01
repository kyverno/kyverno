package policyruleinfo

import (
	"fmt"
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
