package v1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Ready means that the policy is ready
	PolicyConditionReady = "Ready"
)

const (
	// PolicyReasonSucceeded is the reason set when the policy is ready
	PolicyReasonSucceeded = "Succeeded"
	// PolicyReasonSucceeded is the reason set when the policy is not ready
	PolicyReasonFailed = "Failed"
)

// PolicyStatus mostly contains runtime information related to policy execution.
// Deprecated. Policy metrics are now available via the "/metrics" endpoint.
// See: https://kyverno.io/docs/monitoring-kyverno-with-prometheus-metrics/
type PolicyStatus struct {
	// Ready indicates if the policy is ready to serve the admission request.
	// Deprecated in favor of Conditions
	Ready bool `json:"ready" yaml:"ready"`
	// Conditions is a list of conditions that apply to the policy
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Autogen contains autogen status information
	// +optional
	// TODO: finalize status content
	// Autogen AutogenStatus `json:"autogen" yaml:"autogen"`
	// Rules is a list of Rule instances. It contains original rules defined in the spec
	// auto generated rules added for pod controllers
	// TODO: finalize status content
	// Rules []Rule `json:"rules,omitempty" yaml:"rules,omitempty"`
}

func (status *PolicyStatus) SetReady(ready bool) {
	condition := metav1.Condition{
		Type: PolicyConditionReady,
	}
	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = PolicyReasonSucceeded
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = PolicyReasonFailed
	}
	status.Ready = ready
	meta.SetStatusCondition(&status.Conditions, condition)
}

// IsReady indicates if the policy is ready to serve the admission request
func (status *PolicyStatus) IsReady() bool {
	condition := meta.FindStatusCondition(status.Conditions, PolicyConditionReady)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// AutogenStatus contains autogen status information.
// It indicates requested and effective autogen controllers used when
// automatically generating rules.
type AutogenStatus struct {
	// Requested indicates the autogen requested controllers
	Requested []string `json:"requested,omitempty" yaml:"requested,omitempty"`
	// Activated indicates the autogen activated controllers
	Activated []string `json:"activated,omitempty" yaml:"activated,omitempty"`
}
