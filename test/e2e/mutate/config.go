package mutate

// MutateTests is E2E Test Config for mutation
var MutateTests = []struct {
	//TestName - Name of the Test
	TestName string
	// Data - The Yaml file of the ClusterPolicy
	Data []byte
}{
	{
		TestName: "test-mutate-with-context",
		Data:     configMapMutationYaml,
	},
	{
		TestName: "test-mutate-with-logic-in-context",
		Data:     configMapMutationWithContextLogicYaml,
	},
	{
		TestName: "test-mutate-with-context-label-selection",
		Data:     configMapMutationWithContextLabelSelectionYaml,
	},
}
