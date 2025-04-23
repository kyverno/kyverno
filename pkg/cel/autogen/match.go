package autogen

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func CreateMatchConstraints(targets []Target, operations []admissionregistrationv1.OperationType) *admissionregistrationv1.MatchResources {
	rulesMap := map[schema.GroupVersion]sets.Set[string]{}
	for _, target := range targets {
		gv := schema.GroupVersion{Group: target.Group, Version: target.Version}
		resources := rulesMap[gv]
		if resources == nil {
			resources = sets.New[string]()
		}
		resources.Insert(target.Resource)
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
					Resources:   sets.List(resources),
				},
				Operations: operations,
			},
		})
	}
	return &admissionregistrationv1.MatchResources{
		ResourceRules: rules,
	}
}

func CreateMatchConditions(replacements string, targets []Target, conditions []admissionregistrationv1.MatchCondition) []admissionregistrationv1.MatchCondition {
	if len(conditions) == 0 {
		return conditions
	}
	preconditions := sets.New[string]()
	for _, target := range targets {
		apiVersion := target.Group
		if apiVersion != "" {
			apiVersion += "/"
		}
		apiVersion += target.Version
		preconditions = preconditions.Insert(fmt.Sprintf(`(object.apiVersion == '%s' && object.kind =='%s')`, apiVersion, target.Kind))
	}
	precondition := strings.Join(sets.List(preconditions), " || ")
	matchConditions := make([]admissionregistrationv1.MatchCondition, 0, len(conditions))
	prefix := "autogen"
	if replacements != "" {
		prefix = "autogen-" + replacements
	}
	for _, m := range conditions {
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
			Name:       fmt.Sprintf(`%s-%s`, prefix, m.Name),
			Expression: fmt.Sprintf(`!(%s) || (%s)`, precondition, m.Expression),
		})
	}
	return matchConditions
}
