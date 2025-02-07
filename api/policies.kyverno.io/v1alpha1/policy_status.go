package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyConditionType string

const (
	PolicyConditionTypeWebhookConfigured      PolicyConditionType = "WebhookConfigured"
	PolicyConditionTypePolicyCached           PolicyConditionType = "PolicyCached"
	PolicyConditionTypeRBACPermissionsGranted PolicyConditionType = "RBACPermissionsGranted"
)

type PolicyStatus struct {
	// The ready of a policy is a high-level summary of where the policy is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the policy's status.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
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
