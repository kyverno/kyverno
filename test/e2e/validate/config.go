package validate

import (
	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	podGVR       = e2e.GetGVR("", "v1", "pods")
	kustomizeGVR = e2e.GetGVR("kustomize.toolkit.fluxcd.io", "v1beta1", "kustomizations")
)

type ValidationTest struct {
	// TestDescription - Description of the Test
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
}

var FluxValidateTests = []ValidationTest{
	{
		TestDescription:   "test-validate-with-flux-and-variable-substitution-2043",
		PolicyName:        "flux-multi-tenancy",
		PolicyRaw:         kyverno2043Policy,
		ResourceName:      "dev-team",
		ResourceNamespace: "test-validate",
		ResourceGVR:       kustomizeGVR,
		ResourceRaw:       kyverno2043Fluxkustomization,
		MustSucceed:       false,
	},
	{
		TestDescription:   "test-validate-with-flux-and-variable-substitution-2241",
		PolicyName:        "flux-multi-tenancy-2",
		PolicyRaw:         kyverno2241Policy,
		ResourceName:      "tenants",
		ResourceNamespace: "test-validate",
		ResourceGVR:       kustomizeGVR,
		ResourceRaw:       kyverno2241Fluxkustomization,
		MustSucceed:       true,
	},
}

var ValidateTests = []ValidationTest{
	{
		// Case for https://github.com/kyverno/kyverno/issues/2345 issue
		TestDescription:   "checks that contains function works properly with string list",
		PolicyName:        "drop-cap-net-raw",
		PolicyRaw:         kyverno2345Policy,
		ResourceName:      "test",
		ResourceNamespace: "test-validate1",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyverno2345Resource,
		MustSucceed:       false,
	},
	{
		// Case for small image validation
		TestDescription:   "checks that images are small",
		PolicyName:        "check-small-images",
		PolicyRaw:         kyvernoSmallImagePolicy,
		ResourceName:      "pod-with-small-image",
		ResourceNamespace: "test-validate",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyvernoPodWithSmallImage,
		MustSucceed:       true,
	},
	{
		// Case for small image validation
		TestDescription:   "checks that images are small",
		PolicyName:        "check-large-images",
		PolicyRaw:         kyvernoSmallImagePolicy,
		ResourceName:      "pod-with-large-image",
		ResourceNamespace: "test-validate",
		ResourceGVR:       podGVR,
		ResourceRaw:       kyvernoPodWithLargeImage,
		MustSucceed:       false,
	},
}
