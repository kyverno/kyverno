package event

//Source of event generation
type Source int

const (
	// AdmissionController : event generated in admission-control webhook
	AdmissionController Source = iota
	// PolicyController : event generated in policy-controller
	PolicyController
	// GeneratePolicyController : event generated in generate policyController
	GeneratePolicyController
)

func (s Source) String() string {
	return [...]string{
		"kyverno-admission-controller",
		"kyverno-policy-controller",
		"kyverno-generate-policy-controller",
	}[s]
}
