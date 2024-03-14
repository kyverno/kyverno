package validation

import (
	"testing"

	pssutils "github.com/kyverno/kyverno/pkg/pss/utils"
	"github.com/kyverno/kyverno/pkg/utils/api"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"github.com/stretchr/testify/assert"
	"k8s.io/pod-security-admission/policy"
)

var testImages map[string]map[string]api.ImageInfo = map[string]map[string]api.ImageInfo{
	"initContainers": {
		"busybox": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "index.docker.io",
				Name:             "busybox",
				Path:             "busybox",
				Tag:              "v1.2.3",
				Reference:        "index.docker.io/busybox:v1.2.3",
				ReferenceWithTag: "index.docker.io/busybox:v1.2.3",
			},
			Pointer: "/spec/initContainers/0/image",
		},
	},
	"containers": {
		"nginx": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "docker.io",
				Name:             "nginx",
				Path:             "nginx",
				Tag:              "v13.4",
				Reference:        "docker.io/nginx:v13.4",
				ReferenceWithTag: "docker.io/nginx:v13.4",
			},
			Pointer: "/spec/containers/0/image",
		},
	},
	"ephemeralContainers": {
		"nginx2": {
			ImageInfo: imageutils.ImageInfo{
				Registry:         "docker.io",
				Name:             "nginx2",
				Path:             "test/nginx",
				Tag:              "latest",
				Reference:        "docker.io/test/nginx:latest",
				ReferenceWithTag: "docker.io/test/nginx:latest",
			},
			Pointer: "/spec/ephemeralContainers/0/image",
		},
	},
}

var testChecks []pssutils.PSSCheckResult = []pssutils.PSSCheckResult{
	{
		ID: "0",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "containers \"nginx\", \"busybox\" must set securityContext.allowPrivilegeEscalation=false",
		},
	},
	{
		ID: "1",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "containers \"nginx\", \"busybox\" must set securityContext.capabilities.drop=[\"ALL\"]",
		},
	},
	{
		ID: "2",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or containers \"nginx\", \"busybox\" must set securityContext.runAsNonRoot=true",
		},
	},
	{
		ID: "3",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or containers \"nginx\", \"busybox\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\"",
		},
	},
	{
		ID: "4",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "pod or container \"nginx\" must set securityContext.seccompProfile.type to \"RuntimeDefault\" or \"Localhost\"",
		},
	},
	{
		ID: "5",
		CheckResult: policy.CheckResult{
			Allowed:         false,
			ForbiddenReason: "---",
			ForbiddenDetail: "container \"nginx2\" must set securityContext.allowPrivilegeEscalation=false",
		},
	},
}

func Test_addImages(t *testing.T) {
	checks := testChecks
	imageInfos := testImages
	updatedChecks := addImages(checks, imageInfos)

	assert.Equal(t, len(checks), len(updatedChecks))
	assert.Equal(t, []string{"docker.io/nginx:v13.4", "index.docker.io/busybox:v1.2.3"}, updatedChecks[0].Images)
	assert.Equal(t, []string{"docker.io/nginx:v13.4", "index.docker.io/busybox:v1.2.3"}, updatedChecks[1].Images)
	assert.Equal(t, []string{"docker.io/nginx:v13.4", "index.docker.io/busybox:v1.2.3"}, updatedChecks[2].Images)
	assert.Equal(t, []string{"docker.io/nginx:v13.4", "index.docker.io/busybox:v1.2.3"}, updatedChecks[3].Images)
	assert.Equal(t, []string{"docker.io/nginx:v13.4"}, updatedChecks[4].Images)
	assert.Equal(t, []string{"docker.io/test/nginx:latest"}, updatedChecks[5].Images)

	delete(imageInfos, "ephemeralContainers")
	updatedChecks = addImages(checks, imageInfos)
	assert.Equal(t, []string{"nginx2"}, updatedChecks[5].Images)
}
