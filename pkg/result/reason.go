package result

//Reason types of Result Reasons
type Reason int

const (
	//PolicyViolation there is a violation of policy
	Success Reason = iota
	//Success policy applied
	Violation
	//Failed the request to create/update the resource was blocked(generated from admission-controller)
	Failed
)

func (r Reason) String() string {
	return [...]string{
		"Success",
		"Violation",
		"Failed",
	}[r]
}
