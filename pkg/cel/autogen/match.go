package autogen

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
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
	compareGroupVersion := func(gv1 schema.GroupVersion, gv2 schema.GroupVersion) int {
		if x := cmp.Compare(gv1.Group, gv2.Group); x != 0 {
			return x
		}
		if x := cmp.Compare(gv1.Version, gv2.Version); x != 0 {
			return x
		}
		return 0
	}
	for _, gv := range slices.SortedFunc(maps.Keys(rulesMap), compareGroupVersion) {
		resources := rulesMap[gv]
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

func createMatchConditions(prefix string, targets []target, conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1.MatchCondition {
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
	if prefix == "" {
		prefix = "autogen"
	} else {
		prefix = "autogen-" + prefix
	}
	for _, m := range conditions {
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
			Name:       fmt.Sprintf(`%s-%s`, prefix, m.Name),
			Expression: fmt.Sprintf(`!(%s) || (%s)`, precondition, m.Expression),
		})
	}
	return matchConditions
}
