package event

// Reason types of Event Reasons
type Reason int

const (
	PolicyViolation Reason = iota
	PolicyApplied
	PolicyError
	PolicySkipped
)

func (r Reason) String() string {
	return [...]string{
		"PolicyViolation",
		"PolicyApplied",
		"PolicyError",
		"PolicySkipped",
	}[r]
}
