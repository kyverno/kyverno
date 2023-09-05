package processor

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type ResultCounts struct {
	Pass  int
	Fail  int
	Warn  int
	Error int
	Skip  int
}

func updateResultCounts(policy kyvernov1.PolicyInterface, engineResponse *engineapi.EngineResponse, resPath string, rc *ResultCounts, auditWarn bool) {
	printCount := 0
	for _, policyRule := range autogen.ComputeRules(policy) {
		ruleFoundInEngineResponse := false
		for i, ruleResponse := range engineResponse.PolicyResponse.Rules {
			if policyRule.Name == ruleResponse.Name() {
				ruleFoundInEngineResponse = true

				if ruleResponse.Status() == engineapi.RuleStatusPass {
					rc.Pass++
				} else {
					if printCount < 1 {
						fmt.Println("\ninvalid resource", "policy", policy.GetName(), "resource", resPath)
						printCount++
					}
					fmt.Printf("%d. %s - %s\n", i+1, ruleResponse.Name(), ruleResponse.Message())

					if auditWarn && engineResponse.GetValidationFailureAction().Audit() {
						rc.Warn++
					} else {
						rc.Fail++
					}
				}
				continue
			}
		}

		if !ruleFoundInEngineResponse {
			rc.Skip++
		}
	}
}
