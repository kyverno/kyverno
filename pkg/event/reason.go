package event

// Reason types of Event Reasons
type Reason string

const (
	PolicyViolation Reason = "PolicyViolation"
	PolicyApplied   Reason = "PolicyApplied"
	PolicyError     Reason = "PolicyError"
	PolicySkipped   Reason = "PolicySkipped"
)
