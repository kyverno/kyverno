package event

// Source of event generation
type Source string

const (
	// AdmissionController : event generated in admission-control webhook
	AdmissionController Source = "kyverno-admission"
	// PolicyController : event generated in policy-controller
	PolicyController Source = "kyverno-scan"
	// GeneratePolicyController : event generated in generate policyController
	GeneratePolicyController Source = "kyverno-generate"
	// MutateExistingController : event generated for mutateExisting policies
	MutateExistingController Source = "kyverno-mutate"
	// CleanupController : event generated for cleanup policies
	CleanupController Source = "kyverno-cleanup"
)
