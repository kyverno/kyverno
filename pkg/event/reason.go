package event

//Reason types of Event Reasons
type Reason int

const (
	//PolicyViolation there is a violation of policy
	PolicyViolation Reason = iota
	//PolicyApplied policy applied
	PolicyApplied
	//PolicyFailed policy failed
	PolicyFailed
)

func (r Reason) String() string {
	return [...]string{
		"PolicyViolation",
		"PolicyApplied",
		"PolicyFailed",
	}[r]
}
