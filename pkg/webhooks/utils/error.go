package utils

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func GetErrorMsg(engineReponses []engineapi.EngineResponse) string {
	var str []string
	var resourceInfo string
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			// resource in engineReponses is identical as this was called per admission request
			resourceInfo = fmt.Sprintf("%s/%s/%s", er.Resource.GetKind(), er.Resource.GetNamespace(), er.Resource.GetName())
			pol := er.Policy().GetPolicy().(kyvernov1.PolicyInterface)
			str = append(str, fmt.Sprintf("failed policy %s:", pol.GetName()))
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status() != engineapi.RuleStatusPass {
					str = append(str, rule.String())
				}
			}
		}
	}
	return fmt.Sprintf("Resource %s %s", resourceInfo, strings.Join(str, ";"))
}
