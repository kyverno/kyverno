package event

//Reason types of Event Reasons
type Reason int

const (
	//PolicyViolation there is a violation of policy
	PolicyViolation Reason = iota
	//PolicyApplied policy applied
	PolicyApplied
	//RequestBlocked the request to create/update the resource was blocked( generated from admission-controller)
	RequestBlocked
	//PolicyFailed policy failed
	PolicyFailed
)

func (r Reason) String() string {
	return [...]string{
		"PolicyViolation",
		"PolicyApplied",
		"RequestBlocked",
		"PolicyFailed",
	}[r]
}
