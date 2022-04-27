package mutate

import (
	"github.com/blang/semver/v4"
	"github.com/kyverno/kyverno/test/e2e/common"
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
		TestDescription:    "checks that policy mutate env variables of an array with specific index numbers",
		PolicyName:         "add-image-as-env-var",
		PolicyRaw:          kyverno_mutate_json_patch,
		ResourceName:       "foo",
		ResourceNamespace:  "test-mutate-env-array",
		ResourceGVR:        podGVR,
		ResourceRaw:        podWithEnvVar,
		ExpectedPatternRaw: podWithEnvVarPattern,
	},
	{
		TestDescription:    "checks that preconditions are substituted correctly",
		PolicyName:         "replace-docker-hub",
		PolicyRaw:          kyverno_2971_policy,
		ResourceName:       "nginx",
		ResourceNamespace:  "test-mutate-img",
		ResourceGVR:        podGVR,
		ResourceRaw:        kyverno_2971_resource,
		ExpectedPatternRaw: kyverno_2971_pattern,
	},
	{
		TestDescription:    "checks the global anchor variables for emptyDir",
		PolicyName:         "add-safe-to-evict",
		PolicyRaw:          annotate_host_path_policy,
		ResourceName:       "pod-with-emptydir",
		ResourceNamespace:  "emptydir",
		ResourceGVR:        podGVR,
		ResourceRaw:        podWithEmptyDirAsVolume,
		ExpectedPatternRaw: podWithVolumePattern,
	},
	{
		TestDescription:    "checks the global anchor variables for hostPath",
		PolicyName:         "add-safe-to-evict",
		PolicyRaw:          annotate_host_path_policy,
		ResourceName:       "pod-with-hostpath",
		ResourceNamespace:  "hostpath",
		ResourceGVR:        podGVR,
		ResourceRaw:        podWithHostPathAsVolume,
		ExpectedPatternRaw: podWithVolumePattern,
	},
}

var ingressTests = struct {
	testNamespace string
	cpol          []byte
	policyName    string
	tests         []struct {
		testName                          string
		group, version, rsc, resourceName string
		resource                          []byte
		skip                              bool
	}
}{
	testNamespace: "test-ingress",
	cpol:          mutateIngressCpol,
	policyName:    "mutate-ingress-host",
	tests: []struct {
		testName                          string
		group, version, rsc, resourceName string
		resource                          []byte
		skip                              bool
	}{
		{
			testName:     "test-networking-v1-ingress",
			group:        "networking.k8s.io",
			version:      "v1",
			rsc:          "ingresses",
			resourceName: "kuard-v1",
			resource:     ingressNetworkingV1,
			skip:         common.GetKubernetesVersion().LT(semver.MustParse("1.19.0")),
		},
		// the following test can be removed after 1.22 cluster
		{
			testName:     "test-networking-v1beta1-ingress",
			group:        "networking.k8s.io",
			version:      "v1beta1",
			rsc:          "ingresses",
			resourceName: "kuard-v1beta1",
			resource:     ingressNetworkingV1beta1,
			skip:         common.GetKubernetesVersion().GTE(semver.MustParse("1.22.0")),
		},
	},
}

type mutateExistingOperation string

const (
	createTrigger mutateExistingOperation = "createTrigger"
	deleteTrigger mutateExistingOperation = "deleteTrigger"
	createPolicy  mutateExistingOperation = "createPolicy"
)

// Note: sometimes deleting namespaces takes time.
// Using different names for namespaces prevents collisions.
var mutateExistingTests = []struct {
	// TestDescription - Description of the Test
	TestDescription string
	// Operation describes how to trigger the policy
	Operation mutateExistingOperation
	// PolicyName - Name of the Policy
	PolicyName string
	// PolicyRaw - The Yaml file of the ClusterPolicy
	PolicyRaw []byte
	// TriggerName - Name of the Trigger Resource
	TriggerName string
	// TriggerNamespace - Namespace of the Trigger Resource
	TriggerNamespace string
	// TriggerGVR - GVR of the Trigger Resource
	TriggerGVR schema.GroupVersionResource
	// TriggerRaw - The Yaml file of the Trigger Resource
	TriggerRaw []byte
	// TargetName - Name of the Target Resource
	TargetName string
	// TargetNamespace - Namespace of the Target Resource
	TargetNamespace string
	// TargetGVR - GVR of the Target Resource
	TargetGVR schema.GroupVersionResource
	// TargetRaw - The Yaml file of the Target ClusterPolicy
	TargetRaw []byte
	// ExpectedTargetRaw - The Yaml file that contains validate pattern for the expected result
	// This is not the final result. It is just used to validate the result from the engine.
	ExpectedTargetRaw []byte
}{
	{
		TestDescription:   "mutate existing on resource creation",
		Operation:         createTrigger,
		PolicyName:        "test-post-mutation-create-trigger",
		PolicyRaw:         policyCreateTrigger,
		TriggerName:       "dictionary-1",
		TriggerNamespace:  "staging-1",
		TriggerGVR:        configmGVR,
		TriggerRaw:        triggerCreateTrigger,
		TargetName:        "test-secret-1",
		TargetNamespace:   "staging-1",
		TargetGVR:         secretGVR,
		TargetRaw:         targetCreateTrigger,
		ExpectedTargetRaw: expectedTargetCreateTrigger,
	},
	{
		TestDescription:   "mutate existing on resource deletion",
		Operation:         deleteTrigger,
		PolicyName:        "test-post-mutation-delete-trigger",
		PolicyRaw:         policyDeleteTrigger,
		TriggerName:       "dictionary-2",
		TriggerNamespace:  "staging-2",
		TriggerGVR:        configmGVR,
		TriggerRaw:        triggerDeleteTrigger,
		TargetName:        "test-secret-2",
		TargetNamespace:   "staging-2",
		TargetGVR:         secretGVR,
		TargetRaw:         targetDeleteTrigger,
		ExpectedTargetRaw: expectedTargetDeleteTrigger,
	},
	{
		TestDescription:   "mutate existing on policy creation",
		Operation:         createPolicy,
		PolicyName:        "test-post-mutation-create-policy",
		PolicyRaw:         policyCreatePolicy,
		TriggerName:       "dictionary-3",
		TriggerNamespace:  "staging-3",
		TriggerGVR:        configmGVR,
		TriggerRaw:        triggerCreatePolicy,
		TargetName:        "test-secret-3",
		TargetNamespace:   "staging-3",
		TargetGVR:         secretGVR,
		TargetRaw:         targetCreatePolicy,
		ExpectedTargetRaw: expectedTargetCreatePolicy,
	},
	{
		TestDescription:   "mutate existing (patchesJson6902) on resource creation",
		Operation:         createTrigger,
		PolicyName:        "test-post-mutation-json-patch-create-trigger",
		PolicyRaw:         policyCreateTriggerJsonPatch,
		TriggerName:       "dictionary-4",
		TriggerNamespace:  "staging-4",
		TriggerGVR:        configmGVR,
		TriggerRaw:        triggerCreateTriggerJsonPatch,
		TargetName:        "test-secret-4",
		TargetNamespace:   "staging-4",
		TargetGVR:         secretGVR,
		TargetRaw:         targetCreateTriggerJsonPatch,
		ExpectedTargetRaw: expectedCreateTriggerJsonPatch,
	},
}
