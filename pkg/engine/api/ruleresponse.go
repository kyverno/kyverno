package api

import (
	"fmt"
	"time"

	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/pod-security-admission/api"
)

// PodSecurityChecks details about pod securty checks
type PodSecurityChecks struct {
	// Level is the pod security level
	Level api.Level
	// Version is the pod security version
	Version string
	// Checks contains check result details
	Checks []pssutils.PSSCheckResult
}

// RuleResponse details for each rule application
type RuleResponse struct {
	// Name is the rule name specified in policy
	Name string
	// Type is the rule type (Mutation,Generation,Validation) for Kyverno Policy
	Type RuleType
	// Message is the message response from the rule application
	Message string
	// Status rule status
	Status RuleStatus
	// patches are JSON patches, for mutation rules
	patches [][]byte
	// stats contains rule statistics
	stats ExecutionStats
	// generatedResource is the generated by the generate rules of a policy
	generatedResource unstructured.Unstructured
	// patchedTarget is the patched resource for mutate.targets
	patchedTarget *unstructured.Unstructured
	// patchedTargetParentResourceGVR is the GVR of the parent resource of the PatchedTarget. This is only populated when PatchedTarget is a subresource.
	patchedTargetParentResourceGVR metav1.GroupVersionResource
	// patchedTargetSubresourceName is the name of the subresource which is patched, empty if the resource patched is not a subresource.
	patchedTargetSubresourceName string
	// podSecurityChecks contains pod security checks (only if this is a pod security rule)
	podSecurityChecks *PodSecurityChecks
	// exception is the exception applied (if any)
	exception *kyvernov2alpha1.PolicyException
}

func NewRuleResponse(rule string, ruleType RuleType, msg string, status RuleStatus) *RuleResponse {
	return &RuleResponse{
		Name:    rule,
		Type:    ruleType,
		Message: msg,
		Status:  status,
	}
}

func RuleError(rule string, ruleType RuleType, msg string, err error) *RuleResponse {
	if err != nil {
		return NewRuleResponse(rule, ruleType, fmt.Sprintf("%s: %s", msg, err.Error()), RuleStatusError)
	}
	return NewRuleResponse(rule, ruleType, msg, RuleStatusError)
}

func RuleSkip(rule string, ruleType RuleType, msg string) *RuleResponse {
	return NewRuleResponse(rule, ruleType, msg, RuleStatusSkip)
}

func RulePass(rule string, ruleType RuleType, msg string) *RuleResponse {
	return NewRuleResponse(rule, ruleType, msg, RuleStatusPass)
}

func RuleFail(rule string, ruleType RuleType, msg string) *RuleResponse {
	return NewRuleResponse(rule, ruleType, msg, RuleStatusFail)
}

func (r RuleResponse) WithException(exception *kyvernov2alpha1.PolicyException) *RuleResponse {
	r.exception = exception
	return &r
}

func (r RuleResponse) WithPodSecurityChecks(checks PodSecurityChecks) *RuleResponse {
	r.podSecurityChecks = &checks
	return &r
}

func (r RuleResponse) WithPatchedTarget(patchedTarget *unstructured.Unstructured, gvr metav1.GroupVersionResource, subresource string) *RuleResponse {
	r.patchedTarget = patchedTarget
	r.patchedTargetParentResourceGVR = gvr
	r.patchedTargetSubresourceName = subresource
	return &r
}

func (r RuleResponse) WithGeneratedResource(resource unstructured.Unstructured) *RuleResponse {
	r.generatedResource = resource
	return &r
}

func (r RuleResponse) WithPatches(patches ...[]byte) *RuleResponse {
	r.patches = patches
	return &r
}

func (r RuleResponse) WithStats(startTime, endTime time.Time) RuleResponse {
	r.stats = NewExecutionStats(startTime)
	r.stats.Done(endTime)
	return r
}

func (r RuleResponse) Stats() ExecutionStats {
	return r.stats
}

func (r RuleResponse) Exception() *kyvernov2alpha1.PolicyException {
	return r.exception
}

func (r RuleResponse) IsException() bool {
	return r.exception != nil
}

func (r RuleResponse) PodSecurityChecks() *PodSecurityChecks {
	return r.podSecurityChecks
}

func (r RuleResponse) PatchedTarget() (*unstructured.Unstructured, metav1.GroupVersionResource, string) {
	return r.patchedTarget, r.patchedTargetParentResourceGVR, r.patchedTargetSubresourceName
}

func (r RuleResponse) GeneratedResource() unstructured.Unstructured {
	return r.generatedResource
}

func (r RuleResponse) Patches() [][]byte {
	return r.patches
}

// HasStatus checks if rule status is in a given list
func (r RuleResponse) HasStatus(status ...RuleStatus) bool {
	for _, s := range status {
		if r.Status == s {
			return true
		}
	}
	return false
}

// String implements Stringer interface
func (r RuleResponse) String() string {
	return fmt.Sprintf("rule %s (%s): %v", r.Name, r.Type, r.Message)
}
