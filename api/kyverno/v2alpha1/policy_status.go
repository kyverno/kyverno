package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyConditionType string

const (
	PolicyConditionTypeWebhookConfigured      PolicyConditionType = "WebhookConfigured"
	PolicyConditionTypeRBACPermissionsGranted PolicyConditionType = "RBACPermissionsGranted"
)

type PolicyStatus struct {
	// The ready of a policy is a high-level summary of where the policy is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the policy's status.
	// +optional
	Ready *bool `json:"ready,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Autogen AutogenStatus `json:"autogen"`
}

// AutogenStatus contains autogen status information.
type AutogenStatus struct {
	// Rules is a list of Rule instances. It contains auto generated rules added for pod controllers
	Rules []AutogenRule `json:"rules,omitempty"`
}

type AutogenRule struct {
	MatchConstraints *admissionregistrationv1.MatchResources   `json:"matchConstraints,omitempty"`
	MatchConditions  []admissionregistrationv1.MatchCondition  `json:"matchConditions,omitempty"`
	Validations      []admissionregistrationv1.Validation      `json:"validations,omitempty"`
	AuditAnnotation  []admissionregistrationv1.AuditAnnotation `json:"auditAnnotations,omitempty"`
	Variables        []admissionregistrationv1.Variable        `json:"variables,omitempty"`
}

func (status *PolicyStatus) SetReadyByCondition(c PolicyConditionType, s metav1.ConditionStatus, message string) {
	reason := "Succeeded"
	if s != metav1.ConditionTrue {
		reason = "Failed"
	}
	newCondition := metav1.Condition{
		Type:    string(c),
		Reason:  reason,
		Status:  s,
		Message: message,
	}

	meta.SetStatusCondition(&status.Conditions, newCondition)
}
