package autogen

import (
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func createMatchConstraints(targets []target, operations []admissionregistrationv1.OperationType) *admissionregistrationv1.MatchResources {
	rulesMap := map[schema.GroupVersion][]string{}
	for _, target := range targets {
		gv := schema.GroupVersion{Group: target.group, Version: target.version}
		resources := rulesMap[gv]
		resources = append(resources, target.resource)
		rulesMap[gv] = resources
	}
	rules := make([]admissionregistrationv1.NamedRuleWithOperations, 0, len(rulesMap))
	for gv, resources := range rulesMap {
		rules = append(rules, admissionregistrationv1.NamedRuleWithOperations{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{gv.Group},
					APIVersions: []string{gv.Version},
					Resources:   resources,
				},
				Operations: operations,
			},
		})
	}
	return &admissionregistrationv1.MatchResources{
		ResourceRules: rules,
	}
}

func createMatchConditions(targets []target, conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1.MatchCondition {
	var preconditions []string
	for _, target := range targets {
		apiVersion := target.group
		if apiVersion != "" {
			apiVersion += "/"
		}
		apiVersion += target.version
		preconditions = append(preconditions, fmt.Sprintf(`(object.apiVersion == '%s' && object.kind =='%s')`, apiVersion, target.kind))
	}
	precondition := strings.Join(preconditions, "||")

	var matchConditions []admissionregistrationv1.MatchCondition
	for _, m := range conditions {
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
			Name:       m.Name,
			Expression: fmt.Sprintf(`!(%s) || (%s)`, precondition, m.Expression),
		})
	}
	return matchConditions
}
