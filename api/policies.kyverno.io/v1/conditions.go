package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PolicyConditionType string

const (
	PolicyConditionTypeWebhookConfigured      PolicyConditionType = "WebhookConfigured"
	PolicyConditionTypePolicyCached           PolicyConditionType = "PolicyCached"
	PolicyConditionTypeRBACPermissionsGranted PolicyConditionType = "RBACPermissionsGranted"
)

// ConditionStatus is the shared status across all policy types
type ConditionStatus struct {
	// The ready of a policy is a high-level summary of where the policy is in its lifecycle.
	// The conditions array, the reason and message fields contain more detail about the policy's status.
	// +optional
	Ready *bool `json:"ready,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message is a human readable message indicating details about the generation of ValidatingAdmissionPolicy/MutatingAdmissionPolicy
	// It is an empty string when ValidatingAdmissionPolicy/MutatingAdmissionPolicy is successfully generated.
	// +optional
	Message string `json:"message"`
}
