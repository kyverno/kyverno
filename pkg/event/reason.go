package event

// Reason types of Event Reasons
type Reason string

const (
	PolicyViolation Reason = "PolicyViolation"
	PolicyApplied   Reason = "PolicyApplied"
	PolicyError     Reason = "PolicyError"
	PolicySkipped   Reason = "PolicySkipped"
	// LegacyPolicyPresent is reported once at startup when legacy (non-policies.kyverno.io)
	// policy custom resources are still present in the cluster.
	LegacyPolicyPresent Reason = "LegacyPolicyPresent"
)
