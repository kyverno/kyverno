// Package matching evaluates CEL policy match constraints.
//
// # Offline equivalence scope
//
// NewOfflineMatcher provides approximate MatchPolicy=Equivalent matching for the
// file-only CLI. It treats versions of a group/resource and known built-in group
// migrations as equivalent, for both inclusion and exclusion rules. It does not discover
// served versions or convert objects. If an inclusion match requires converting
// an object, it returns an error before CEL evaluation. Equivalent exclusions
// need no conversion because they prevent evaluation. Exact matching remains strict.
//
// # Admission limitation
//
// NewMatcher is also used by the in-cluster engines. Without an injected mapper,
// it only matches the supplied request resource, even when MatchPolicy is
// Equivalent or omitted. The CLI's equivalence support does not implement full
// Kubernetes Equivalent semantics for these production callers.
//
// API-server webhook conversion does not remove this limitation. Kyverno can
// group several policies' rules into one webhook. The API server chooses a
// representation for that webhook, not a separate representation for each
// policy. A request can therefore reach Kyverno but be skipped by a policy whose
// rule names another equivalent version. Equivalent exclusions can likewise be
// missed. These remain runtime limitations.
//
// NewMatcherWithEquivalentResourceMapper supports authoritative equivalence
// lookup, but neither returns a target version nor converts objects. Inclusion
// matches requiring object conversion return an error. Full runtime support
// requires wiring authoritative resource equivalence together with per-policy
// conversion before CEL evaluation and propagating conversion failures through
// the existing policy error flow. Merely
// injecting a mapper, or enabling the offline approximation in admission, is not
// sufficient.
package matching

import (
	"fmt"
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
	objectMatcher            *object.Matcher
	equivalentResourceMapper runtime.EquivalentResourceMapper
	offline                  bool
}

// NewMatcher matches only the requested resource unless an authoritative
// equivalent-resource mapper is supplied with NewMatcherWithEquivalentResourceMapper.
// Equivalent and an omitted MatchPolicy do not enable cross-version matching
// here. See the package's admission limitation; file-only CLI callers use NewOfflineMatcher.
func NewMatcher() Matcher {
	return NewMatcherWithEquivalentResourceMapper(nil)
}

// NewOfflineMatcher enables approximate equivalence for file-only CLI evaluation
// without API-server registration data. It must not be used for in-cluster or
// cluster-backed CLI evaluation.
// It neither verifies that a version is served nor converts objects for CEL.
// Inclusion matches requiring object conversion return an explicit error.
func NewOfflineMatcher() Matcher {
	return &matcher{objectMatcher: &object.Matcher{}, offline: true}
}

// NewMatcherWithEquivalentResourceMapper uses the supplied mapper as the authority
// for resource equivalence. A nil mapper does not perform equivalent matching.
// It does not convert objects to a matching rule's version; inclusion matches
// requiring conversion return an error. This is not a full Equivalent admission
// implementation.
func NewMatcherWithEquivalentResourceMapper(mapper runtime.EquivalentResourceMapper) Matcher {
	return &matcher{
		objectMatcher:            &object.Matcher{},
		equivalentResourceMapper: mapper,
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
	mapper := m.resourceMapper(attr, matchResources)
	if isExcluded, err := matchesResourceRules(matchResources.ExcludeResourceRules, attr, matchResources.MatchPolicy, mapper, false); isExcluded || err != nil {
		return false, err
	}

	var (
		isMatch  bool
		matchErr error
	)
	if len(matchResources.ResourceRules) == 0 {
		isMatch = true
	} else {
		isMatch, matchErr = matchesResourceRules(matchResources.ResourceRules, attr, matchResources.MatchPolicy, mapper, true)
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

func (m *matcher) resourceMapper(attr admission.Attributes, resources admissionregistrationv1.MatchResources) runtime.EquivalentResourceMapper {
	if m.offline {
		return localEquivalentResourceMapper(attr, resources)
	}
	return m.equivalentResourceMapper
}

func matchesResourceRules(namedRules []admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes, matchPolicy *admissionregistrationv1.MatchPolicyType, mapper runtime.EquivalentResourceMapper, forEvaluation bool) (bool, error) {
	// Prefer an exact match across all rules before considering conversion.
	for _, namedRule := range namedRules {
		ruleMatcher := rules.Matcher{
			Rule: namedRule.RuleWithOperations,
			Attr: attr,
		}
		if ruleMatcher.Matches() && matchesResourceName(namedRule, attr) {
			return true, nil
		}
	}
	for _, namedRule := range namedRules {
		resource, matches := matchesEquivalentVersion(attr, namedRule.RuleWithOperations, matchPolicy, mapper)
		if !matches || !matchesResourceName(namedRule, attr) {
			continue
		}
		if forEvaluation && (attr.GetObject() != nil || attr.GetOldObject() != nil) {
			kind := mapper.KindFor(resource, attr.GetSubresource())
			needsConversion := kind != attr.GetKind()
			if kind.Empty() {
				needsConversion = resource != attr.GetResource()
			}
			if needsConversion {
				return false, fmt.Errorf("cannot evaluate equivalent resource %s as %s: API-version conversion is unavailable; provide the resource in the policy rule's API version", attr.GetResource(), resource)
			}
		}
		return true, nil
	}
	return false, nil
}

func matchesResourceName(namedRule admissionregistrationv1.NamedRuleWithOperations, attr admission.Attributes) bool {
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

// matchesEquivalentVersion checks the mapper's equivalent resources without
// relaxing the rule. An unset MatchPolicy follows the default of Equivalent.
func matchesEquivalentVersion(attr admission.Attributes, rule admissionregistrationv1.RuleWithOperations, matchPolicy *admissionregistrationv1.MatchPolicyType, mapper runtime.EquivalentResourceMapper) (schema.GroupVersionResource, bool) {
	if mapper == nil || (matchPolicy != nil && *matchPolicy == admissionregistrationv1.Exact) {
		return schema.GroupVersionResource{}, false
	}
	for _, resource := range mapper.EquivalentResourcesFor(attr.GetResource(), attr.GetSubresource()) {
		equivalentAttr := &attributesWithResource{Attributes: attr, resource: resource}
		equivalentMatcher := rules.Matcher{Rule: rule, Attr: equivalentAttr}
		if equivalentMatcher.Matches() {
			return resource, true
		}
	}
	return schema.GroupVersionResource{}, false
}

type attributesWithResource struct {
	admission.Attributes
	resource schema.GroupVersionResource
}

func (a *attributesWithResource) GetResource() schema.GroupVersionResource {
	return a.resource
}

// localEquivalentResourceMapper seeds a request-local Kubernetes registry from
// the request and rules. Versions of the same group/resource (including CRDs) and
// known built-in API migrations share an identity. It is never used to supplement
// an injected mapper. Only resource matching is needed, not object conversion.
func localEquivalentResourceMapper(attr admission.Attributes, resources admissionregistrationv1.MatchResources) runtime.EquivalentResourceMapper {
	registry := runtime.NewEquivalentResourceRegistryWithIdentity(func(resource schema.GroupResource) string {
		return equivalentGroupResource(resource).String()
	})
	resource := attr.GetResource()
	registry.RegisterKindFor(resource, attr.GetSubresource(), schema.GroupVersionKind{})
	for _, namedRules := range [][]admissionregistrationv1.NamedRuleWithOperations{resources.ResourceRules, resources.ExcludeResourceRules} {
		for _, rule := range namedRules {
			for _, group := range rule.APIGroups {
				if group == "*" {
					group = resource.Group
				}
				for _, version := range rule.APIVersions {
					if version == "*" {
						version = resource.Version
					}
					candidate := schema.GroupVersionResource{Group: group, Version: version, Resource: resource.Resource}
					registry.RegisterKindFor(candidate, attr.GetSubresource(), schema.GroupVersionKind{})
				}
			}
		}
	}
	return registry
}

// equivalentGroupResource maps only known cross-group equivalents to a shared
// identity. Unrelated APIs with the same resource plural retain their group.
// These identities follow Kubernetes' default_storage_factory_builder.go.
func equivalentGroupResource(resource schema.GroupResource) schema.GroupResource {
	if resource.Group == "extensions" {
		switch resource.Resource {
		case "deployments", "daemonsets", "replicasets":
			resource.Group = "apps"
		case "ingresses", "networkpolicies":
			resource.Group = "networking.k8s.io"
		case "replicationcontrollers":
			resource.Group = ""
		case "podsecuritypolicies":
			resource.Group = "policy"
		}
	}
	if resource.Group == "" && resource.Resource == "events" {
		resource.Group = "events.k8s.io"
	}
	return resource
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
