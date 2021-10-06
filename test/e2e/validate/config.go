package validate

import (
	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FluxValidateTests is E2E Test Config for validation
var FluxValidateTests = []struct {
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

var podGVR = e2e.GetGVR("", "v1", "pods")

var ValidateTests = []struct {
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
	// MustSucceed - indicates if validation must succeed
	MustSucceed bool
}{
	{
		// Case for https://github.com/kyverno/kyverno/issues/2345 issue
		TestDescription:   "checks that contains function works properly with string list",
		PolicyName:        "drop-cap-net-raw",
		PolicyRaw:         kyverno_2345_policy,
		ResourceName:      "test",
		ResourceNamespace: "test-validate1",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyverno_2345_resource,
		MustSucceed:       false,
	},
	{
		// Case for https://github.com/kyverno/kyverno/issues/2390 issue
		TestDescription:   "checks that policy contains global anchor fields",
		PolicyName:        "check-image-pull-secret",
		PolicyRaw:         kyverno_global_anchor_validate_policy,
		ResourceName:      "pod-with-nginx-allowed-registory",
		ResourceNamespace: "",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyverno_global_anchor_validate_resource_1,
		MustSucceed:       true,
	},
	{
		// Case for https://github.com/kyverno/kyverno/issues/2390 issue
		TestDescription:   "checks that policy contains global anchor fields",
		PolicyName:        "check-image-pull-secret",
		PolicyRaw:         kyverno_global_anchor_validate_policy,
		ResourceName:      "pod-with-nginx-disallowed-registory",
		ResourceNamespace: "",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyverno_global_anchor_validate_resource_2,
		MustSucceed:       false,
	},
}
