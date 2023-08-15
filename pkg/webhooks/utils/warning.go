package utils

import (
	"fmt"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func GetWarningMessages(engineResponses []engineapi.EngineResponse) []string {
	var warnings []string
	for _, er := range engineResponses {
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status() != engineapi.RuleStatusPass && rule.Status() != engineapi.RuleStatusSkip {
				msg := fmt.Sprintf("policy %s.%s: %s", er.Policy().GetName(), rule.Name(), rule.Message())
				warnings = append(warnings, msg)
			}
		}
	}
	return warnings
}
