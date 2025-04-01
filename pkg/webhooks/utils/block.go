package utils

import (
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	"gopkg.in/yaml.v2"
)

func getAction(hasViolations bool, i int) string {
	action := "error"
	if hasViolations {
		action = "violation"
	}
	if i > 1 {
		action = action + "s"
	}
	return action
}

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func BlockRequest(engineResponses []engineapi.EngineResponse, failurePolicy kyvernov1.FailurePolicyType, log logr.Logger) bool {
	// First process non-deferred enforcement policies
	for _, er := range engineResponses {
		// Skip DeferEnforce actions for now, we'll process them after evaluating all policies
		if er.GetValidationFailureAction().DeferEnforcement() {
			continue
		}

		if engineutils.BlockRequest(er, failurePolicy) {
			log.V(2).Info("blocking admission request", "policy", er.Policy().GetName())
			return true
		}
	}

	// Now check if any DeferEnforce policies have failed
	for _, er := range engineResponses {
		if er.GetValidationFailureAction().DeferEnforcement() && er.IsFailed() {
			log.V(2).Info("blocking admission request with deferred enforcement", "policy", er.Policy().GetName())
			return true
		}
	}

	log.V(4).Info("allowing admission request")
	return false
}

// GetBlockedMessages gets the error messages for rules with error or fail status
func GetBlockedMessages(engineResponses []engineapi.EngineResponse) string {
	if len(engineResponses) == 0 {
		return ""
	}
	failures := make(map[string]interface{})
	for _, er := range engineResponses {
		ruleToReason := make(map[string]string)
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status() != engineapi.RuleStatusPass && rule.Status() != engineapi.RuleStatusSkip {
				ruleToReason[rule.Name()] = rule.Message()
			}
		}
		if len(ruleToReason) != 0 {
			failures[er.Policy().GetName()] = ruleToReason
		}
	}
	if len(failures) == 0 {
		return ""
	}
	r := engineResponses[0].Resource
	resourceName := fmt.Sprintf("%s/%s/%s", r.GetKind(), r.GetNamespace(), r.GetName())
	results, _ := yaml.Marshal(failures)
	msg := fmt.Sprintf("\n\nresource %s was blocked due to the following policies \n\n%s", resourceName, results)
	return msg
}
