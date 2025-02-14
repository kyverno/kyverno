package v1alpha1

import (
	admissionregistrationvlalpha1 "k8s.io/api/admissionregistration/v1alpha1"
)

// MutatingPolicySpec is the specification of the desired behavior of the MutatingPolicy.
type MutatingPolicySpec struct {
	admissionregistrationvlalpha1.MutatingAdmissionPolicySpec `json:",inline"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`
}
