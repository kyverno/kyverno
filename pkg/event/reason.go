package event

//Reason types of Event Reasons
type Reason int

const (
	PolicyViolation Reason = iota
	PolicyApplied
	PolicyError
)

func (r Reason) String() string {
	return [...]string{
		"PolicyViolation",
		"PolicyApplied",
		"PolicyError",
	}[r]
}
