package mutate

// MutateTests is E2E Test Config for mutation
var MutateTests = []struct {
	//TestName - Name of the Test
	TestName string
	// Data - The Yaml file of the ClusterPolicy
	Data []byte
	// ResourceNamespace - Namespace of the Resource
	ResourceNamespace string
	// PolicyName - Name of the Policy
	PolicyName string
}{
	{
		TestName:          "test-mutate-with-context",
		Data:              configMapMutationYaml,
		ResourceNamespace: "test-mutate",
		PolicyName:        "mutate-policy",
	},
	{
		TestName:          "test-mutate-with-logic-in-context",
		Data:              configMapMutationWithContextLogicYaml,
		ResourceNamespace: "test-mutate",
		PolicyName:        "mutate-policy",
	},
	{
		TestName:          "test-mutate-with-context-label-selection",
		Data:              configMapMutationWithContextLabelSelectionYaml,
		ResourceNamespace: "test-mutate",
		PolicyName:        "mutate-policy",
	},
}

var ingressTests = struct {
	testNamesapce string
	cpol          []byte
	policyName    string
	tests         []struct {
		testName                          string
		group, version, rsc, resourceName string
		resource                          []byte
	}
}{
	testNamesapce: "test-ingress",
	cpol:          mutateIngressCpol,
	policyName:    "mutate-ingress-host",
	tests: []struct {
		testName                          string
		group, version, rsc, resourceName string
		resource                          []byte
	}{
		{
			testName:     "test-networking-v1-ingress",
			group:        "networking.k8s.io",
			version:      "v1",
			rsc:          "ingresses",
			resourceName: "kuard-v1",
			resource:     ingressNetworkingV1,
		},
		// the following two tests can be removed after 1.22 cluster
		{
			testName:     "test-networking-v1beta1-ingress",
			group:        "networking.k8s.io",
			version:      "v1beta1",
			rsc:          "ingresses",
			resourceName: "kuard-v1beta1",
			resource:     ingressNetworkingV1beta1,
		},
		{
			testName:     "test-extensions-v1beta1-ingress",
			group:        "extensions",
			version:      "v1beta1",
			rsc:          "ingresses",
			resourceName: "kuard-extensions",
			resource:     ingressExtensionV1beta1,
		},
	},
}
