package matching

import (
	"strings"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/log"
	"github.com/kyverno/kyverno/pkg/cel/matching/predicates/namespace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	if isExcluded, err := matchesResourceRules(matchResources.ExcludeResourceRules, matchResources.MatchPolicy, attr); isExcluded || err != nil {
		return false, err
	}

	var (
		isMatch  bool
		matchErr error
	)
	if len(matchResources.ResourceRules) == 0 {
		isMatch = true
	} else {
		isMatch, matchErr = matchesResourceRules(matchResources.ResourceRules, matchResources.MatchPolicy, attr)
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

func matchesResourceRules(namedRules []admissionregistrationv1.NamedRuleWithOperations, matchPolicy *admissionregistrationv1.MatchPolicyType, attr admission.Attributes) (bool, error) {
	for _, namedRule := range namedRules {
		rule := namedRule.RuleWithOperations
		ruleMatcher := rules.Matcher{
			Rule: rule,
			Attr: attr,
		}
		if ruleMatcher.Matches() {
			if matchesName(namedRule, attr) {
				return true, nil
			}
			continue
		}

		// If matchPolicy is undefined or exact, don't perform equivalent matching
		if matchPolicy == nil || *matchPolicy != admissionregistrationv1.Equivalent {
			continue
		}

		if matchesEquivalentResourceRule(namedRule, attr) {
			return true, nil
		}
	}
	return false, nil
}

func matchesName(namedRule admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) bool {
	// an empty name list always matches
	if len(namedRule.ResourceNames) == 0 {
		return true
	}

	name := attr.GetName()

	if name == "" {
		if matchesGeneratedName(namedRule, attr) {
			return true
		}

		log.Log.V(2).Info("Skipping name match due to empty name and no matching generateName", "resource", attr)
		return false
	}

	for _, matchedName := range namedRule.ResourceNames {
		if name == matchedName {
			return true
		}
	}
	return false
}

type attrWithResourceOverride struct {
	admission.Attributes
	resource schema.GroupVersionResource
}

func (a *attrWithResourceOverride) GetResource() schema.GroupVersionResource {
	return a.resource
}

var equivalentGroupMap = map[string]map[string][]string{
	"deployments": {
		"apps":       {"extensions"},
		"extensions": {"apps"},
	},
	"daemonsets": {
		"apps":       {"extensions"},
		"extensions": {"apps"},
	},
	"replicasets": {
		"apps":       {"extensions"},
		"extensions": {"apps"},
	},
	"ingresses": {
		"networking.k8s.io": {"extensions"},
		"extensions":        {"networking.k8s.io"},
	},
	"networkpolicies": {
		"networking.k8s.io": {"extensions"},
		"extensions":        {"networking.k8s.io"},
	},
	"podsecuritypolicies": {
		"policy":     {"extensions"},
		"extensions": {"policy"},
	},
}

func matchesEquivalentResourceRule(namedRule admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) bool {
	if !matchesName(namedRule, attr) {
		return false
	}

	attrRes := attr.GetResource()
	candidateGroups := []string{attrRes.Group}
	if equivGroups, ok := equivalentGroupMap[attrRes.Resource][attrRes.Group]; ok {
		candidateGroups = append(candidateGroups, equivGroups...)
	}

	rule := namedRule.RuleWithOperations
	for _, candidateGroup := range candidateGroups {
		for _, candidateVersion := range rule.APIVersions {
			if candidateGroup == attrRes.Group && candidateVersion == attrRes.Version {
				continue
			}
			override := &attrWithResourceOverride{
				Attributes: attr,
				resource: schema.GroupVersionResource{
					Group:    candidateGroup,
					Version:  candidateVersion,
					Resource: attrRes.Resource,
				},
			}
			m := rules.Matcher{
				Rule: rule,
				Attr: override,
			}
			if m.Matches() {
				return true
			}
		}
	}
	return false
}

func matchesGeneratedName(rule admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) bool {
	obj := attr.GetObject()
	if obj == nil {
		return false
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	genName := accessor.GetGenerateName()
	for _, matchedName := range rule.ResourceNames {
		if strings.HasPrefix(genName, matchedName) {
			return true
		}
	}

	return false
}
