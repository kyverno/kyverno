package api

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	utils "github.com/kyverno/kyverno/pkg/utils/match"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EngineResponse engine response to the action
type EngineResponse struct {
	// Resource is the original resource
	Resource unstructured.Unstructured
	// policy is the original policy
	policy kyvernov1.PolicyInterface
	// namespaceLabels given by policy context
	namespaceLabels map[string]string
	// PatchedResource is the resource patched with the engine action changes
	PatchedResource unstructured.Unstructured
	// PolicyResponse contains the engine policy response
	PolicyResponse PolicyResponse
	// stats contains engine statistics
	stats ExecutionStats
}

func resource(policyContext PolicyContext) unstructured.Unstructured {
	resource := policyContext.NewResource()
	if resource.Object == nil {
		resource = policyContext.OldResource()
	}
	return resource
}

func NewEngineResponseFromPolicyContext(policyContext PolicyContext) EngineResponse {
	return NewEngineResponse(
		resource(policyContext),
		policyContext.Policy(),
		policyContext.NamespaceLabels(),
	)
}

func NewEngineResponse(
	resource unstructured.Unstructured,
	policy kyvernov1.PolicyInterface,
	namespaceLabels map[string]string,
) EngineResponse {
	return EngineResponse{
		Resource:        resource,
		policy:          policy,
		namespaceLabels: namespaceLabels,
		PatchedResource: resource,
	}
}

func (er EngineResponse) WithPolicy(policy kyvernov1.PolicyInterface) EngineResponse {
	er.policy = policy
	return er
}

func (er EngineResponse) WithPolicyResponse(policyResponse PolicyResponse) EngineResponse {
	er.PolicyResponse = policyResponse
	return er
}

func (r EngineResponse) WithStats(stats ExecutionStats) EngineResponse {
	r.stats = stats
	return r
}

func (er EngineResponse) WithPatchedResource(patchedResource unstructured.Unstructured) EngineResponse {
	er.PatchedResource = patchedResource
	return er
}

func (er EngineResponse) WithNamespaceLabels(namespaceLabels map[string]string) EngineResponse {
	er.namespaceLabels = namespaceLabels
	return er
}

func (er *EngineResponse) NamespaceLabels() map[string]string {
	return er.namespaceLabels
}

func (er *EngineResponse) Policy() kyvernov1.PolicyInterface {
	return er.policy
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
	return datautils.DeepEqual(er, EngineResponse{})
}

// GetPatches returns all the patches joined
func (er EngineResponse) GetPatches() [][]byte {
	var patches [][]byte
	for _, r := range er.PolicyResponse.Rules {
		patches = append(patches, r.Patches()...)
	}
	return patches
}

// GetFailedRules returns failed rules
func (er EngineResponse) GetFailedRules() []string {
	return er.getRules(func(rule RuleResponse) bool { return rule.HasStatus(RuleStatusFail, RuleStatusError) })
}

// GetFailedRulesWithErrors returns failed rules with corresponding error messages
func (er EngineResponse) GetFailedRulesWithErrors() []string {
	return er.getRulesWithErrors(func(rule RuleResponse) bool { return rule.HasStatus(RuleStatusFail, RuleStatusError) })
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
			rules = append(rules, r.Name())
		}
	}
	return rules
}

func (er EngineResponse) getRulesWithErrors(predicate func(RuleResponse) bool) []string {
	var rules []string
	for _, r := range er.PolicyResponse.Rules {
		if predicate(r) {
			rules = append(rules, fmt.Sprintf("%s: %s", r.Name(), r.Message()))
		}
	}
	return rules
}

func (er EngineResponse) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	spec := er.Policy().GetSpec()
	for _, v := range spec.ValidationFailureActionOverrides {
		if !v.Action.IsValid() {
			continue
		}
		if v.Namespaces == nil {
			hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.namespaceLabels)
			if err == nil && hasPass {
				return v.Action
			}
		}
		for _, ns := range v.Namespaces {
			if wildcard.Match(ns, er.PatchedResource.GetNamespace()) {
				if v.NamespaceSelector == nil {
					return v.Action
				}
				hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.namespaceLabels)
				if err == nil && hasPass {
					return v.Action
				}
			}
		}
	}
	return spec.ValidationFailureAction
}
