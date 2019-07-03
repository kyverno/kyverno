package webhooks

import (
	"strings"

	"github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
)

//StringInSlice checks if string is present in slice of strings
func StringInSlice(kind string, list []string) bool {
	for _, b := range list {
		if b == kind {
			return true
		}
	}
	return false
}

//parseKinds parses the kinds if a single string contains comma seperated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(list []string) []string {
	kinds := []string{}
	for _, k := range list {
		args := strings.Split(k, ",")
		for _, arg := range args {
			if arg != "" {
				kinds = append(kinds, strings.TrimSpace(arg))
			}
		}
	}
	return kinds
}

type ArrayFlags []string

func (i *ArrayFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// extract the kinds that the policy rules apply to
func getApplicableKindsForPolicy(p *v1alpha1.Policy) []string {
	kindsMap := map[string]interface{}{}
	kinds := []string{}
	// iterate over the rules an identify all kinds
	for _, rule := range p.Spec.Rules {
		for _, k := range rule.ResourceDescription.Kinds {
			kindsMap[k] = nil
		}
	}

	// get the kinds
	for k := range kindsMap {
		kinds = append(kinds, k)
	}
	return kinds
}

func contains(ruleNames []string, ruleName string) bool {
	for _, rn := range ruleNames {
		if rn == ruleName {
			return true
		}
	}
	return false
}
