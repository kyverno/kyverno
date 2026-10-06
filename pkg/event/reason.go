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
	// OrphanedWebhookConfigPresent is reported once at startup when webhook configuration
	// objects from an old Kyverno install, no longer created or used by this version, are
	// still present in the cluster.
	OrphanedWebhookConfigPresent Reason = "OrphanedWebhookConfigPresent"
)
