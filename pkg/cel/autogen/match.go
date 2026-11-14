package autogen

import (
	"cmp"
	"crypto/sha256"
	"fmt"
	"maps"
	"slices"
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func CreateMatchConstraints(targets []policiesv1beta1.Target, operations []admissionregistrationv1.OperationType, namespaceSelector *metav1.LabelSelector) *admissionregistrationv1.MatchResources {
	if len(targets) == 0 {
		return nil
	}
	if len(operations) == 0 {
		return nil
	}
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
	var selector *metav1.LabelSelector
	if namespaceSelector != nil {
		selector = namespaceSelector.DeepCopy()
	}
	return &admissionregistrationv1.MatchResources{
		ResourceRules:     rules,
		NamespaceSelector: selector,
	}
}

func CreateMatchConditions(config string, targets []policiesv1beta1.Target, conditions []admissionregistrationv1.MatchCondition, used ...sets.Set[string]) []admissionregistrationv1.MatchCondition {
	if len(targets) == 0 {
		return nil
	}
	if len(conditions) == 0 {
		return conditions
	}
	// Extraction targets' Spec (and therefore its MatchConditions) is never
	// rewritten - it's still written against a Pod (see
	// ExtractionReplacementsRef). Wrapping it here would make it an
	// API-server-level matchCondition evaluated against the raw parent
	// object (e.g. a JobSet), before Kyverno's webhook - and before
	// extraction ever runs - so it would almost always spuriously fail and
	// silently prevent the webhook from being invoked at all. The
	// corresponding ResourceRules (added independently by the caller) already
	// scope the webhook to the right GVR/operations; the policy's own
	// MatchConditions are correctly re-evaluated inside the engine, once per
	// synthesized Pod, after extraction.
	if config == ExtractionReplacementsRef {
		return nil
	}
	// The target is identified from request.kind, not from object: the API server binds object to
	// null for DELETE, and a precondition that errors there makes every wrapped condition error on
	// a non-matching DELETE, which rejects the request under failurePolicy Fail and skips Kyverno
	// under Ignore. request.kind is set for every operation.
	preconditions := sets.New[string]()
	for _, target := range targets {
		preconditions = preconditions.Insert(fmt.Sprintf(`(request.kind.group == '%s' && request.kind.version == '%s' && request.kind.kind == '%s')`, target.Group, target.Version, target.Kind))
	}
	precondition := strings.Join(sets.List(preconditions), " || ")
	matchConditions := make([]admissionregistrationv1.MatchCondition, 0, len(conditions))
	var usedNames sets.Set[string]
	if len(used) > 0 && used[0] != nil {
		usedNames = used[0]
	} else {
		usedNames = sets.New[string]()
	}
	prefix := "autogen"
	if config != "" {
		prefix = "autogen-" + config
	}
	for _, m := range conditions {
		expr := m.Expression
		if strings.Contains(expr, "//") && !strings.HasSuffix(expr, "\n") {
			expr += "\n"
		}
		name := formatMatchConditionName(prefix, m.Name, usedNames)
		usedNames.Insert(name)
		matchConditions = append(matchConditions, admissionregistrationv1.MatchCondition{
			Name:       name,
			Expression: fmt.Sprintf(`!(%s) || (%s)`, precondition, expr),
		})
	}
	return matchConditions
}

const maxMatchConditionNameLength = 63

func formatMatchConditionName(prefix, name string, used sets.Set[string]) string {
	fullName := fmt.Sprintf(`%s-%s`, prefix, name)
	if len(fullName) <= maxMatchConditionNameLength && (used == nil || !used.Has(fullName)) {
		return fullName
	}

	maxBaseLen := maxMatchConditionNameLength - 8 - 1 // 54

	for counter := 0; ; counter++ {
		var hash string
		if counter == 0 {
			hash = fmt.Sprintf("%x", sha256.Sum256([]byte(fullName)))[:8]
		} else {
			hash = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s#%d", fullName, counter))))[:8]
		}

		base := fullName
		if len(base) > maxBaseLen {
			base = base[:maxBaseLen]
		}
		trimmed := strings.TrimRight(base, "-_./")
		cand := fmt.Sprintf(`%s-%s`, trimmed, hash)
		if used == nil || !used.Has(cand) {
			return cand
		}
	}
}
