package v2alpha1

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// ValidatingPolicySpec is the specification of the desired behavior of the ValidatingPolicy.
type ValidatingPolicySpec struct {
	Spec admissionregistrationv1.ValidatingAdmissionPolicySpec `json:",inline"`

	// WebhookConfiguration defines the configuration for the webhook.
	// +optional
	WebhookConfiguration *WebhookConfiguration `json:"webhookConfiguration,omitempty"`
}

type WebhookConfiguration struct {
	// FailurePolicy defines how unexpected policy errors and webhook response timeout errors are handled.
	// Allowed values are Ignore or Fail. Defaults to Fail.
	// +optional
	FailurePolicy *admissionregistrationv1.FailurePolicyType `json:"failurePolicy,omitempty"`

	// TimeoutSeconds specifies the maximum time in seconds allowed to apply this policy.
	// After the configured time expires, the admission request may fail, or may simply ignore the policy results,
	// based on the failure policy. The default timeout is 10s, the value must be between 1 and 30 seconds.
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// MatchCondition configures admission webhook matchConditions.
	// Requires Kubernetes 1.27 or later.
	// +optional
	MatchConditions []admissionregistrationv1.MatchCondition `json:"matchConditions,omitempty"`

	// matchPolicy defines how the matching resources are used to match incoming requests.
	// This field can be overridden by the matchPolicy field in the matchConstraints.
	// Allowed values are "Exact" or "Equivalent". Defaults to "Equivalent".
	// +optional
	MatchPolicy *admissionregistrationv1.MatchPolicyType `json:"matchPolicy,omitempty"`
}
