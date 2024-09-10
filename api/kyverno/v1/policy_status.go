package v1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PolicyConditionReady means that the policy is ready
	PolicyConditionReady = "Ready"
)

const (
	// PolicyReasonSucceeded is the reason set when the policy is ready
	PolicyReasonSucceeded = "Succeeded"
	// PolicyReasonSucceeded is the reason set when the policy is not ready
	PolicyReasonFailed = "Failed"
)

// Deprecated. Policy metrics are now available via the "/metrics" endpoint.
// See: https://kyverno.io/docs/monitoring-kyverno-with-prometheus-metrics/
type PolicyStatus struct {
	// Deprecated in favor of Conditions
	Ready *bool `json:"ready,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// +optional
	Autogen AutogenStatus `json:"autogen"`
	// +optional
	RuleCount RuleCountStatus `json:"rulecount"`
	// ValidatingAdmissionPolicy contains status information
	// +optional
	ValidatingAdmissionPolicy ValidatingAdmissionPolicyStatus `json:"validatingadmissionpolicy"`
}

// RuleCountStatus contains four variables which describes counts for
// validate, generate, mutate and verify images rules
type RuleCountStatus struct {
	// Count for validate rules in policy
	Validate int `json:"validate"`
	// Count for generate rules in policy
	Generate int `json:"generate"`
	// Count for mutate rules in policy
	Mutate int `json:"mutate"`
	// Count for verify image rules in policy
	VerifyImages int `json:"verifyimages"`
}

func (status *PolicyStatus) SetReady(ready bool, message string) {
	condition := metav1.Condition{
		Type:    PolicyConditionReady,
		Message: message,
	}
	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = PolicyReasonSucceeded
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = PolicyReasonFailed
	}
	status.Ready = nil
	meta.SetStatusCondition(&status.Conditions, condition)
}

// IsReady indicates if the policy is ready to serve the admission request
func (status *PolicyStatus) IsReady() bool {
	condition := meta.FindStatusCondition(status.Conditions, PolicyConditionReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// AutogenStatus contains autogen status information.
type AutogenStatus struct {
	// Rules is a list of Rule instances. It contains auto generated rules added for pod controllers
	Rules []Rule `json:"rules,omitempty"`
}

// ValidatingAdmissionPolicy contains status information
type ValidatingAdmissionPolicyStatus struct {
	// Generated indicates whether a validating admission policy is generated from the policy or not
	Generated bool `json:"generated"`
	// Message is a human readable message indicating details about the generation of validating admission policy
	// It is an empty string when validating admission policy is successfully generated.
	Message string `json:"message"`
}
