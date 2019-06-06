package result

//Reason types of Result Reasons
type Reason int

const (
	//Success policy applied
	Success Reason = iota
	//Violation there is a violation of policy
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
