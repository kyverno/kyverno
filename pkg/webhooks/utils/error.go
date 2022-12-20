package utils

import (
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/pkg/engine/response"
)

func GetErrorMsg(engineReponses []*response.EngineResponse) string {
	var str []string
	var resourceInfo string
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			// resource in engineReponses is identical as this was called per admission request
			resourceInfo = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			str = append(str, fmt.Sprintf("failed policy %s:", er.PolicyResponse.Policy.Name))
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					str = append(str, rule.ToString())
				}
			}
		}
	}
	return fmt.Sprintf("Resource %s %s", resourceInfo, strings.Join(str, ";"))
}
