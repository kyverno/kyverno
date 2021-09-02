package validate

// ValidateTests is E2E Test Config for validation
var ValidateTests = []struct {
	//TestName - Name of the Test
	TestName string
	// PolicyRaw - The Yaml file of the ClusterPolicy
	PolicyRaw []byte
	// ResourceRaw - The Yaml file of the ClusterPolicy
	ResourceRaw []byte
	// ResourceNamespace - Namespace of the Resource
	ResourceNamespace string
	// MustSucceed declares if test case must fail on validation
	MustSucceed bool
}{
	{
		TestName:          "test-validate-with-flux-and-variable-substitution-2043",
		PolicyRaw:         kyverno_2043_policy,
		ResourceRaw:       kyverno_2043_FluxKustomization,
		ResourceNamespace: "test-validate",
		MustSucceed:       false,
	},
	{
		TestName:          "test-validate-with-flux-and-variable-substitution-2241",
		PolicyRaw:         kyverno_2241_policy,
		ResourceRaw:       kyverno_2241_FluxKustomization,
		ResourceNamespace: "test-validate",
		MustSucceed:       true,
	},
}
