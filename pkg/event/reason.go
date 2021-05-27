package event

//Reason types of Event Reasons
type Reason int

const (
	//PolicyViolation there is a violation of policy
	PolicyViolation Reason = iota
	//PolicyFailed policy failed
	PolicyFailed
)

func (r Reason) String() string {
	return [...]string{
		"PolicyViolation",
		"PolicyFailed",
	}[r]
}
