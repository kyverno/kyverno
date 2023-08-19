package utils

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/api/admissionregistration/v1alpha1"
)

func GetWarningMessages(engineResponses []engineapi.EngineResponse) []string {
	var warnings []string
	for _, er := range engineResponses {
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status() != engineapi.RuleStatusPass && rule.Status() != engineapi.RuleStatusSkip {
				pol := er.Policy()
				if polType := pol.GetType(); polType == engineapi.ValidatingAdmissionPolicyType {
					vap := pol.GetPolicy().(v1alpha1.ValidatingAdmissionPolicy)	
					msg := fmt.Sprintf("policy %s.%s: %s", vap.GetName(), rule.Name(), rule.Message())
					warnings = append(warnings, msg)
				} else {
					kyvernopol := er.Policy().GetPolicy().(kyvernov1.PolicyInterface)
					msg := fmt.Sprintf("policy %s.%s: %s", kyvernopol.GetName(), rule.Name(), rule.Message())
					warnings = append(warnings, msg)
				}
			}
		}
	}
	return warnings
}
