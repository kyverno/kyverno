package validate

// ValidateTests is E2E Test Config for validation
var ValidateTests = []struct {
	//TestName - Name of the Test
	TestName string
	// Data - The Yaml file of the ClusterPolicy
	Data []byte
	// ResourceNamespace - Namespace of the Resource
	ResourceNamespace string
}{
	{
		TestName:          "test-validate-with-flux-and-variable-substitution",
		Data:              kyverno_2043_policy,
		ResourceNamespace: "test-validate",
	},
}
