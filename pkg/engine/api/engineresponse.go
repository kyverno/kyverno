package api

import (
	"reflect"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	utils "github.com/kyverno/kyverno/pkg/utils/match"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EngineResponse engine response to the action
type EngineResponse struct {
	// Resource is the original resource
	Resource unstructured.Unstructured
	// Policy is the original policy
	Policy kyvernov1.PolicyInterface
	// NamespaceLabels given by policy context
	NamespaceLabels map[string]string
	// PatchedResource is the resource patched with the engine action changes
	PatchedResource unstructured.Unstructured
	// PolicyResponse contains the engine policy response
	PolicyResponse PolicyResponse
}

func Resource(policyContext PolicyContext) unstructured.Unstructured {
	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	return resource
}

func NewEngineResponseFromPolicyContext(
	policyContext PolicyContext,
	policyResponse *PolicyResponse,
) *EngineResponse {
	return NewEngineResponse(
		Resource(policyContext),
		policyContext.Policy(),
		policyContext.NamespaceLabels(),
		policyResponse,
	)
}

func NewEngineResponse(
	resource unstructured.Unstructured,
	policy kyvernov1.PolicyInterface,
	namespaceLabels map[string]string,
	policyResponse *PolicyResponse,
) *EngineResponse {
	response := &EngineResponse{
		Resource:        resource,
		Policy:          policy,
		NamespaceLabels: namespaceLabels,
	}
	if policyResponse != nil {
		response.PolicyResponse = *policyResponse
	}
	return response
}

// IsOneOf checks if any rule has status in a given list
func (er EngineResponse) IsOneOf(status ...RuleStatus) bool {
	for _, r := range er.PolicyResponse.Rules {
		if r.HasStatus(status...) {
			return true
		}
	}
	return false
}

// IsSuccessful checks if any rule has failed or produced an error during execution
func (er EngineResponse) IsSuccessful() bool {
	return !er.IsOneOf(RuleStatusFail, RuleStatusError)
}

// IsSkipped checks if any rule has skipped resource or not.
func (er EngineResponse) IsSkipped() bool {
	return er.IsOneOf(RuleStatusSkip)
}

// IsFailed checks if any rule created a policy violation
func (er EngineResponse) IsFailed() bool {
	return er.IsOneOf(RuleStatusFail)
}

// IsError checks if any rule resulted in a processing error
func (er EngineResponse) IsError() bool {
	return er.IsOneOf(RuleStatusError)
}

// IsEmpty checks if any rule results are present
func (er EngineResponse) IsEmpty() bool {
	return len(er.PolicyResponse.Rules) == 0
}

// isNil checks if rule is an empty rule
func (er EngineResponse) IsNil() bool {
	return reflect.DeepEqual(er, EngineResponse{})
}

// GetPatches returns all the patches joined
func (er EngineResponse) GetPatches() [][]byte {
	var patches [][]byte
	for _, r := range er.PolicyResponse.Rules {
		if r.Patches != nil {
			patches = append(patches, r.Patches...)
		}
	}
	return patches
}

// GetFailedRules returns failed rules
func (er EngineResponse) GetFailedRules() []string {
	return er.getRules(func(rule RuleResponse) bool { return rule.HasStatus(RuleStatusFail, RuleStatusError) })
}

// GetSuccessRules returns success rules
func (er EngineResponse) GetSuccessRules() []string {
	return er.getRules(func(rule RuleResponse) bool { return rule.HasStatus(RuleStatusPass) })
}

// GetResourceSpec returns resourceSpec of er
func (er EngineResponse) GetResourceSpec() ResourceSpec {
	return ResourceSpec{
		Kind:       er.PatchedResource.GetKind(),
		APIVersion: er.PatchedResource.GetAPIVersion(),
		Namespace:  er.PatchedResource.GetNamespace(),
		Name:       er.PatchedResource.GetName(),
		UID:        string(er.PatchedResource.GetUID()),
	}
}

func (er EngineResponse) getRules(predicate func(RuleResponse) bool) []string {
	var rules []string
	for _, r := range er.PolicyResponse.Rules {
		if predicate(r) {
			rules = append(rules, r.Name)
		}
	}
	return rules
}

func (er *EngineResponse) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	for _, v := range er.PolicyResponse.ValidationFailureActionOverrides {
		if !v.Action.IsValid() {
			continue
		}
		if v.Namespaces == nil {
			hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.NamespaceLabels)
			if err == nil && hasPass {
				return v.Action
			}
		}
		for _, ns := range v.Namespaces {
			if wildcard.Match(ns, er.PatchedResource.GetNamespace()) {
				if v.NamespaceSelector == nil {
					return v.Action
				}
				hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.NamespaceLabels)
				if err == nil && hasPass {
					return v.Action
				}
			}
		}
	}
	return er.PolicyResponse.ValidationFailureAction
}
