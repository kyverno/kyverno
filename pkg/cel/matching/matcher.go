package matching

import (
	"github.com/kyverno/kyverno/pkg/cel/matching/predicates/namespace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules"
)

type Matcher interface {
	Match(matching.MatchCriteria, admission.Attributes, runtime.Object) (bool, error)
}

type matcher struct {
	objectMatcher *object.Matcher
}

func NewMatcher() Matcher {
	return &matcher{
		objectMatcher: &object.Matcher{},
	}
}

func (m *matcher) Match(criteria matching.MatchCriteria, attr admission.Attributes, ns runtime.Object) (bool, error) {
	nsMatcher := namespace.Matcher{
		Namespace: ns,
	}
	matches, matchNsErr := nsMatcher.MatchNamespaceSelector(criteria, attr)
	// Should not return an error here for policy which do not apply to the request, even if err is an unexpected scenario.
	if !matches && matchNsErr == nil {
		return false, nil
	}

	matches, matchObjErr := m.objectMatcher.MatchObjectSelector(criteria, attr)
	// Should not return an error here for policy which do not apply to the request, even if err is an unexpected scenario.
	if !matches && matchObjErr == nil {
		return false, nil
	}

	matchResources := criteria.GetMatchResources()
	if isExcluded, err := matchesResourceRules(matchResources.ExcludeResourceRules, attr); isExcluded || err != nil {
		return false, err
	}

	var (
		isMatch  bool
		matchErr error
	)
	if len(matchResources.ResourceRules) == 0 {
		isMatch = true
	} else {
		isMatch, matchErr = matchesResourceRules(matchResources.ResourceRules, attr)
	}
	if matchErr != nil {
		return false, matchErr
	}
	if !isMatch {
		return false, nil
	}

	// now that we know this applies to this request otherwise, if there were selector errors, return them
	if matchNsErr != nil {
		return false, matchNsErr
	}
	if matchObjErr != nil {
		return false, matchObjErr
	}

	return true, nil
}

func matchesResourceRules(namedRules []admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) (bool, error) {
	for _, namedRule := range namedRules {
		rule := namedRule.RuleWithOperations
		ruleMatcher := rules.Matcher{
			Rule: rule,
			Attr: attr,
		}
		if !ruleMatcher.Matches() {
			continue
		}
		// an empty name list always matches
		if len(namedRule.ResourceNames) == 0 {
			return true, nil
		}
		// TODO: GetName() can return an empty string if the user is relying on
		// the API server to generate the name... figure out what to do for this edge case
		name := attr.GetName()
		for _, matchedName := range namedRule.ResourceNames {
			if name == matchedName {
				return true, nil
			}
		}
	}
	return false, nil
}
