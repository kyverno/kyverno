package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// ValidatingPolicySpec is the specification of the desired behavior of the ValidatingPolicy.
type ValidatingPolicySpec struct {
	admissionregistrationv1.ValidatingAdmissionPolicySpec `json:",inline"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`
}

type WebhookConfiguration struct {
	// TimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}
