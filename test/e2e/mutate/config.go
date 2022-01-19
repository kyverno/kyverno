package mutate

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

// Note: sometimes deleting namespaces takes time.
// Using different names for namespaces prevents collisions.
var tests = []struct {
	//TestDescription - Description of the Test
	TestDescription string
	// PolicyName - Name of the Policy
	PolicyName string
	// PolicyRaw - The Yaml file of the ClusterPolicy
	PolicyRaw []byte
	// ResourceName - Name of the Resource
	ResourceName string
	// ResourceNamespace - Namespace of the Resource
	ResourceNamespace string
	// ResourceGVR - GVR of the Resource
	ResourceGVR schema.GroupVersionResource
	// ResourceRaw - The Yaml file of the ClusterPolicy
	ResourceRaw []byte
	// ExpectedPatternRaw - The Yaml file that contains validate pattern for the expected result
	// This is not the final result. It is just used to validate the result from the engine.
	ExpectedPatternRaw []byte
}{
	{
		TestDescription:    "checks that runAsNonRoot is added to security context and containers elements security context",
		PolicyName:         "set-runasnonroot-true",
		PolicyRaw:          setRunAsNonRootTrue,
		ResourceName:       "foo",
		ResourceNamespace:  "test-mutate",
		ResourceGVR:        podGVR,
		ResourceRaw:        podWithContainers,
		ExpectedPatternRaw: podWithContainersPattern,
	},
	{
		TestDescription:    "checks that runAsNonRoot is added to security context and containers elements security context and initContainers elements security context",
		PolicyName:         "set-runasnonroot-true",
		PolicyRaw:          setRunAsNonRootTrue,
		ResourceName:       "foo",
		ResourceNamespace:  "test-mutate1",
		ResourceGVR:        podGVR,
		ResourceRaw:        podWithContainersAndInitContainers,
		ExpectedPatternRaw: podWithContainersAndInitContainersPattern,
	},
	{
		TestDescription:    "checks that variables in the keys are working correctly",
		PolicyName:         "structured-logs-sidecar",
		PolicyRaw:          kyverno_2316_policy,
		ResourceName:       "busybox",
		ResourceNamespace:  "test-mutate2",
		ResourceGVR:        deploymentGVR,
		ResourceRaw:        kyverno_2316_resource,
		ExpectedPatternRaw: kyverno_2316_pattern,
	},
	{
		TestDescription:    "checks that preconditions are substituted correctly",
		PolicyName:         "replace-docker-hub",
		PolicyRaw:          kyverno_2971_policy,
		ResourceName:       "nginx",
		ResourceNamespace:  "test-mutate",
		ResourceGVR:        podGVR,
		ResourceRaw:        kyverno_2971_resource,
		ExpectedPatternRaw: kyverno_2971_pattern,
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
		// the following test can be removed after 1.22 cluster
		{
			testName:     "test-networking-v1beta1-ingress",
			group:        "networking.k8s.io",
			version:      "v1beta1",
			rsc:          "ingresses",
			resourceName: "kuard-v1beta1",
			resource:     ingressNetworkingV1beta1,
		},
	},
}
