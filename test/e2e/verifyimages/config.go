package verifyimages

import (
	"github.com/kyverno/kyverno/test/e2e"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var taskGVR = e2e.GetGVR("tekton.dev", "v1beta1", "tasks")

var VerifyImagesTests = []struct {
	// TestName - Name of the Test
	TestName string
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
		// Case for custom image extraction
		TestName:          "checks that custom images are populated with simple extractor",
		PolicyName:        "tasks-simple",
		PolicyRaw:         kyvernoTaskPolicyWithSimpleExtractor,
		ResourceName:      "example-task-name",
		ResourceNamespace: "test-verify-images",
		ResourceGVR:       taskGVR,
		ResourceRaw:       tektonTask,
		MustSucceed:       false,
	},
	{
		// Case for custom image extraction
		TestName:          "checks that custom images are populated with complex extractor",
		PolicyName:        "tasks-complex",
		PolicyRaw:         kyvernoTaskPolicyWithComplexExtractor,
		ResourceName:      "example-task-name",
		ResourceNamespace: "test-verify-images",
		ResourceGVR:       taskGVR,
		ResourceRaw:       tektonTask,
		MustSucceed:       false,
	},
	{
		// Case for custom image extraction
		TestName:          "checks that custom images are not populated",
		PolicyName:        "tasks-no-extractor",
		PolicyRaw:         kyvernoTaskPolicyWithoutExtractor,
		ResourceName:      "example-task-name",
		ResourceNamespace: "test-verify-images",
		ResourceGVR:       taskGVR,
		ResourceRaw:       tektonTask,
		MustSucceed:       true,
	},
	{
		// Case for custom image extraction
		TestName:          "checks that custom images are populated and verified",
		PolicyName:        "tasks-keyless",
		PolicyRaw:         kyvernoTaskPolicyKeyless,
		ResourceName:      "example-task-name",
		ResourceNamespace: "test-verify-images",
		ResourceGVR:       taskGVR,
		ResourceRaw:       tektonTaskVerified,
		MustSucceed:       true,
	},
	{
		// Case for custom image extraction
		TestName:          "checks that custom images are populated and verified for all images",
		PolicyName:        "tasks-keyless-required",
		PolicyRaw:         kyvernoTaskPolicyKeylessRequired,
		ResourceName:      "example-task-name",
		ResourceNamespace: "test-verify-images",
		ResourceGVR:       taskGVR,
		ResourceRaw:       tektonTaskVerified,
		MustSucceed:       true,
	},
}
