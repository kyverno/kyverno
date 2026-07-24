package api

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/ext/wildcard"
	"github.com/kyverno/kyverno/pkg/autogen"
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

// matchOverride returns the action of the first override whose namespaces and namespace
// selector match the resource, and whether any override matched.
func (er EngineResponse) matchOverride(overrides []kyvernov1.ValidationFailureActionOverride) (kyvernov1.ValidationFailureAction, bool) {
	for _, v := range overrides {
		if !v.Action.IsValid() {
			continue
		}
		if v.Namespaces == nil {
			// Both Namespaces and NamespaceSelector nil means the override applies to all namespaces.
			if v.NamespaceSelector == nil {
				return v.Action, true
			}
			if hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.namespaceLabels); err == nil && hasPass {
				return v.Action, true
			}
		}
		for _, ns := range v.Namespaces {
			if wildcard.Match(ns, er.PatchedResource.GetNamespace()) {
				if v.NamespaceSelector == nil {
					return v.Action, true
				}
				if hasPass, err := utils.CheckSelector(v.NamespaceSelector, er.namespaceLabels); err == nil && hasPass {
					return v.Action, true
				}
			}
		}
	}
	return "", false
}

// ruleAction returns the rule's own validation failure action and whether the rule sets one.
// It is resolved from the rule's failureActionOverrides, then its failureAction, then its
// verifyImages entries (Enforce wins if any entry enforces).
func (er EngineResponse) ruleAction(r kyvernov1.Rule) (kyvernov1.ValidationFailureAction, bool) {
	if r.HasValidate() {
		if action, ok := er.matchOverride(r.Validation.FailureActionOverrides); ok {
			return action, true
		}
		if r.Validation.FailureAction != nil {
			return *r.Validation.FailureAction, true
		}
	} else if r.HasVerifyImages() {
		var firstAction *kyvernov1.ValidationFailureAction
		for i := range r.VerifyImages {
			if r.VerifyImages[i].FailureAction != nil {
				if r.VerifyImages[i].FailureAction.Enforce() {
					return *r.VerifyImages[i].FailureAction, true
				}
				if firstAction == nil {
					firstAction = r.VerifyImages[i].FailureAction
				}
			}
		}
		if firstAction != nil {
			return *firstAction, true
		}
	}
	return "", false
}

// GetValidationFailureAction returns a single failure action for the whole policy: the first
// rule that sets an explicit action, otherwise the spec-level override or spec-level action.
// An empty string is returned for a ValidatingAdmissionPolicy.
func (er EngineResponse) GetValidationFailureAction() kyvernov1.ValidationFailureAction {
	pol := er.Policy()
	if pol.AsKyvernoPolicy() == nil {
		return ""
	}
	spec := pol.AsKyvernoPolicy().GetSpec()
	for _, r := range spec.Rules {
		if action, ok := er.ruleAction(r); ok {
			return action
		}
	}
	if action, ok := er.matchOverride(spec.ValidationFailureActionOverrides); ok {
		return action
	}
	return spec.ValidationFailureAction
}

// failureActionForRule resolves the effective failure action for the rule with the given name
// within the supplied rule set, falling back to the spec-level override and action when that
// rule sets none.
func (er EngineResponse) failureActionForRule(spec *kyvernov1.Spec, rules []kyvernov1.Rule, name string) kyvernov1.ValidationFailureAction {
	for _, r := range rules {
		if r.Name != name {
			continue
		}
		if action, ok := er.ruleAction(r); ok {
			return action
		}
		break
	}
	if action, ok := er.matchOverride(spec.ValidationFailureActionOverrides); ok {
		return action
	}
	return spec.ValidationFailureAction
}

// HasEnforcedFailure reports whether any rule that failed resolves to an Enforce action. The
// block decision uses this instead of a single policy-wide action, so a failing Enforce rule
// blocks regardless of its position in the policy, and a failing Audit rule does not block even
// when the policy also contains Enforce rules. Rules are resolved against the autogenerated rule
// set so names match the rules actually evaluated (autogen renames rules for pod controllers).
func (er EngineResponse) HasEnforcedFailure() bool {
	pol := er.Policy()
	if pol.AsKyvernoPolicy() == nil {
		return false
	}
	kpol := pol.AsKyvernoPolicy()
	spec := kpol.GetSpec()
	var rules []kyvernov1.Rule
	computed := false
	for i := range er.PolicyResponse.Rules {
		rule := er.PolicyResponse.Rules[i]
		if rule.Status() != RuleStatusFail {
			continue
		}
		if !computed {
			rules = autogen.Default.ComputeRules(kpol, er.Resource.GetKind())
			computed = true
		}
		if er.failureActionForRule(spec, rules, rule.Name()).Enforce() {
			return true
		}
	}
	return false
}
