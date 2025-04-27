package matching

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/policy/matching"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/namespace"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object"
	"k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules"
)

type Matcher interface {
	Match(criteria matching.MatchCriteria, attr admission.Attributes, namespace runtime.Object) (bool, error)
}

func NewMatcher() Matcher {
	return &matcher{}
}

type matcher struct{}

func (e *matcher) Match(criteria matching.MatchCriteria, attr admission.Attributes, namespace runtime.Object) (bool, error) {
	matches, matchNsErr := matchNamespace(criteria, namespace)
	// Should not return an error here for policy which do not apply to the request, even if err is an unexpected scenario.
	if !matches && matchNsErr == nil {
		return false, nil
	}
	matches, matchObjErr := matchObject(criteria, attr)
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

func matchNamespace(provider namespace.NamespaceSelectorProvider, namespace runtime.Object) (bool, error) {
	selector, err := provider.GetParsedNamespaceSelector()
	if err != nil {
		return false, err
	}
	if selector.Empty() {
		return true, nil
	}
	if namespace == nil {
		// If the request is about a cluster scoped resource, and it is not a
		// namespace, it is never exempted.
		return true, nil
	}
	accessor, err := meta.Accessor(namespace)
	if err != nil {
		return false, err
	}
	return selector.Matches(labels.Set(accessor.GetLabels())), nil
}

func _matchObject(obj runtime.Object, selector labels.Selector) bool {
	if obj == nil {
		return false
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	return selector.Matches(labels.Set(accessor.GetLabels()))
}

func matchObject(provider object.ObjectSelectorProvider, attr admission.Attributes) (bool, error) {
	selector, err := provider.GetParsedObjectSelector()
	if err != nil {
		return false, err
	}
	if selector.Empty() {
		return true, nil
	}
	return _matchObject(attr.GetObject(), selector) || _matchObject(attr.GetOldObject(), selector), nil
}

func matchesResourceRules(namedRules []admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) (bool, error) {
	for _, namedRule := range namedRules {
		ruleMatcher := rules.Matcher{
			Rule: namedRule.RuleWithOperations,
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
