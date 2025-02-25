package v1alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// ValidatingPolicySpec is the specification of the desired behavior of the ValidatingPolicy.
type ValidatingPolicySpec struct {
	admissionregistrationv1.ValidatingAdmissionPolicySpec `json:",inline"`

	// ValidationAction specifies the action to be taken when the matched resource violates the policy.
	// Required.
	// +listType=set
	ValidationAction []admissionregistrationv1.ValidationAction `json:"validationActions,omitempty"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`

	// EvaluationConfiguration defines the configuration for the policy evaluation.
	// +optional
	EvaluationConfiguration *EvaluationConfiguration `json:"evaluationConfiguration,omitempty"`
}

// AdmissionEnabled checks if admission is set to true
func (s ValidatingPolicySpec) AdmissionEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Admission == nil || s.EvaluationConfiguration.Admission.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Admission.Enabled
}

// BackgroundEnabled checks if background is set to true
func (s ValidatingPolicySpec) BackgroundEnabled() bool {
	if s.EvaluationConfiguration == nil || s.EvaluationConfiguration.Background == nil || s.EvaluationConfiguration.Background.Enabled == nil {
		return true
	}
	return *s.EvaluationConfiguration.Background.Enabled
}

type WebhookConfiguration struct {
	// TimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

type EvaluationConfiguration struct {
	// Admission controls policy evaluation during admission.
	// +optional
	Admission *AdmissionConfiguration `json:"admission,omitempty"`

	// Background  controls policy evaluation during background scan.
	// +optional
	Background *BackgroundConfiguration `json:"background,omitempty"`
}

type AdmissionConfiguration struct {
	// Enabled controls if rules are applied during admission.
	// Optional. Default value is "true".
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}

type BackgroundConfiguration struct {
	// Enabled controls if rules are applied to existing resources during a background scan.
	// Optional. Default value is "true". The value must be set to "false" if the policy rule
	// uses variables that are only available in the admission review request (e.g. user name).
	// +optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`
}
