package api

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	utils "github.com/kyverno/kyverno/pkg/utils/match"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EngineResponse engine response to the action
type EngineResponse struct {
	// Resource is the original resource
	Resource unstructured.Unstructured
	// Policy is the original policy
	policy GenericPolicy
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
		NewKyvernoPolicy(policyContext.Policy()),
		policyContext.NamespaceLabels(),
	)
}

func NewEngineResponse(
	resource unstructured.Unstructured,
	policy GenericPolicy,
	namespaceLabels map[string]string,
) EngineResponse {
	return EngineResponse{
		Resource:        resource,
		policy:          policy,
		namespaceLabels: namespaceLabels,
		PatchedResource: resource,
	}
}

func (er EngineResponse) WithPolicy(policy GenericPolicy) EngineResponse {
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

func (er EngineResponse) WithWarning() EngineResponse {
	er.PolicyResponse.emitWarning = true
	return er
}

func (er *EngineResponse) NamespaceLabels() map[string]string {
	return er.namespaceLabels
}

func (er *EngineResponse) Policy() GenericPolicy {
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

// IsNil checks if rule is an empty rule
func (er EngineResponse) IsNil() bool {
	return datautils.DeepEqual(er, EngineResponse{})
}

// EmitsWarning checks if policy emits warnings
func (er EngineResponse) EmitsWarning() bool {
	return er.PolicyResponse.emitWarning
}

// GetPatches returns all the patches joined
func (er EngineResponse) GetPatches() []jsonpatch.JsonPatchOperation {
	originalBytes, err := er.Resource.MarshalJSON()
	if err != nil {
		return nil
	}
	patchedBytes, err := er.PatchedResource.MarshalJSON()
	if err != nil {
		return nil
	}
	patches, err := jsonpatch.CreatePatch(originalBytes, patchedBytes)
	if err != nil {
		return nil
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
	if er.PatchedResource.Object == nil {
		return ResourceSpec{
			Kind:       er.Resource.GetKind(),
			APIVersion: er.Resource.GetAPIVersion(),
			Namespace:  er.Resource.GetNamespace(),
			Name:       er.Resource.GetName(),
			UID:        string(er.Resource.GetUID()),
		}
	}

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

// matchesFailureActionOverride reports whether a validationFailureAction override applies
// to the given resource namespace and namespace labels.
func matchesFailureActionOverride(v kyvernov1.ValidationFailureActionOverride, namespaceLabels map[string]string, resourceNamespace string) bool {
	if !v.Action.IsValid() {
		return false
	}
	if v.Namespaces == nil {
		// If both Namespaces and NamespaceSelector are nil, the override applies to all namespaces
		if v.NamespaceSelector == nil {
			return true
		}
		hasPass, err := utils.CheckSelector(v.NamespaceSelector, namespaceLabels)
		return err == nil && hasPass
	}
	for _, ns := range v.Namespaces {
		if wildcard.Match(ns, resourceNamespace) {
			if v.NamespaceSelector == nil {
				return true
			}
			hasPass, err := utils.CheckSelector(v.NamespaceSelector, namespaceLabels)
			if err == nil && hasPass {
				return true
			}
		}
	}
	return false
}

// ResolveValidationFailureAction resolves the effective failureAction for a single
// validate rule. It honors the rule's own failureActionOverrides and failureAction
// first, then falls back to the deprecated policy-level overrides and
// spec.validationFailureAction. Scoping the decision to a single rule ensures that a
// policy mixing Audit and Enforce validate rules honors each rule's action
// independently of its position in the policy (see issue #16557).
func ResolveValidationFailureAction(validation *kyvernov1.Validation, spec *kyvernov1.Spec, namespaceLabels map[string]string, resourceNamespace string) kyvernov1.ValidationFailureAction {
	if validation != nil {
		for _, v := range validation.FailureActionOverrides {
			if matchesFailureActionOverride(v, namespaceLabels, resourceNamespace) {
				return v.Action
			}
		}
		if validation.FailureAction != nil {
			return *validation.FailureAction
		}
	}
	for _, v := range spec.ValidationFailureActionOverrides {
		if matchesFailureActionOverride(v, namespaceLabels, resourceNamespace) {
			return v.Action
		}
	}
	return spec.ValidationFailureAction
}

// EnforcesFailedRules reports whether any failed rule should block the admission request,
// i.e. its effective failureAction resolves to Enforce. Validate rules stamped by the
// engine carry their own per-rule action, so an Enforce rule blocks regardless of its
// position relative to Audit rules in the same policy. Failed rules without a per-rule
// action (mutate rules, image verification, or responses built outside the engine) fall
// back to the policy-level action, preserving existing behavior (see issue #16557).
func (er EngineResponse) EnforcesFailedRules() bool {
	var policyAction *kyvernov1.ValidationFailureAction
	for _, rule := range er.PolicyResponse.Rules {
		if !rule.HasStatus(RuleStatusFail) {
			continue
		}
		if action := rule.ValidationFailureAction(); action != nil {
			if action.Enforce() {
				return true
			}
			continue
		}
		if policyAction == nil {
			a := er.GetValidationFailureAction()
			policyAction = &a
		}
		if policyAction.Enforce() {
			return true
		}
	}
	return false
}

// If the policy is of type ValidatingAdmissionPolicy, an empty string is returned.
func (er EngineResponse) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	pol := er.Policy()
	if pol.AsKyvernoPolicy() == nil {
		return ""
	}
	spec := pol.AsKyvernoPolicy().GetSpec()
	for _, r := range spec.Rules {
		if r.HasValidate() {
			for _, v := range r.Validation.FailureActionOverrides {
				if matchesFailureActionOverride(v, er.namespaceLabels, er.PatchedResource.GetNamespace()) {
					return v.Action
				}
			}
			if r.Validation.FailureAction != nil {
				return *r.Validation.FailureAction
			}
		} else if r.HasVerifyImages() {
			// Check all VerifyImages entries - if ANY has Enforce, return Enforce
			// This ensures that enforcement is not bypassed when multiple entries exist
			var firstAction *kyvernov1.ValidationFailureAction
			for i := range r.VerifyImages {
				if r.VerifyImages[i].FailureAction != nil {
					if r.VerifyImages[i].FailureAction.Enforce() {
						return *r.VerifyImages[i].FailureAction
					}
					if firstAction == nil {
						firstAction = r.VerifyImages[i].FailureAction
					}
				}
			}
			// If no Enforce found but we have an explicit action, return it
			if firstAction != nil {
				return *firstAction
			}
		}
	}
	for _, v := range spec.ValidationFailureActionOverrides {
		if matchesFailureActionOverride(v, er.namespaceLabels, er.PatchedResource.GetNamespace()) {
			return v.Action
		}
	}
	return spec.ValidationFailureAction
}
