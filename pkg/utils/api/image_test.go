package api

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
)

var cfg = config.NewDefaultConfiguration(false)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		raw    []byte
		images map[string]map[string]ImageInfo
	}{
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}], "ephemeralContainers": [{"name": "ephemeral", "image":"test/nginx:latest"}]}}`),
			images: map[string]map[string]ImageInfo{
				"initContainers": {
					"init": {
						imageutils.ImageInfo{
							Registry:         "index.docker.io",
							Name:             "busybox",
							Path:             "busybox",
							Tag:              "v1.2.3",
							Reference:        "index.docker.io/busybox:v1.2.3",
							ReferenceWithTag: "index.docker.io/busybox:v1.2.3",
						},
						"/spec/initContainers/0/image",
						[]string{},
					},
				},
				"containers": {
					"nginx": {
						imageutils.ImageInfo{
							Registry:         "docker.io",
							Name:             "nginx",
							Path:             "nginx",
							Tag:              "latest",
							Reference:        "docker.io/nginx:latest",
							ReferenceWithTag: "docker.io/nginx:latest",
						},
						"/spec/containers/0/image",
						[]string{},
					},
				},
				"ephemeralContainers": {
					"ephemeral": {
						imageutils.ImageInfo{
							Registry:         "docker.io",
							Name:             "nginx",
							Path:             "test/nginx",
							Tag:              "latest",
							Reference:        "docker.io/test/nginx:latest",
							ReferenceWithTag: "docker.io/test/nginx:latest",
						},
						"/spec/ephemeralContainers/0/image",
						[]string{},
					},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"containers": [{"name": "nginx","image": "test/nginx:latest"}]}}`),
			images: map[string]map[string]ImageInfo{
				"containers": {
					"nginx": {
						imageutils.ImageInfo{
							Registry:         "docker.io",
							Name:             "nginx",
							Path:             "test/nginx",
							Tag:              "latest",
							Reference:        "docker.io/test/nginx:latest",
							ReferenceWithTag: "docker.io/test/nginx:latest",
						},
						"/spec/containers/0/image",
						[]string{},
					},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "myapp"},"spec": {"selector": {"matchLabels": {"app": "myapp"}},"template": {"metadata": {"labels": {"app": "myapp"}},"spec": {"initContainers": [{"name": "init","image": "fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers": [{"name": "myapp","image": "fictional.registry.example:10443/imagename"}],"ephemeralContainers": [{"name": "ephemeral","image": "fictional.registry.example:10443/imagename:tag@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}] }}}}`),
			images: map[string]map[string]ImageInfo{
				"initContainers": {
					"init": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "tag",
							Digest:           "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
							Reference:        "fictional.registry.example:10443/imagename@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:tag",
						},
						"/spec/template/spec/initContainers/0/image",
						[]string{},
					},
				},
				"containers": {
					"myapp": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "latest",
							Reference:        "fictional.registry.example:10443/imagename:latest",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:latest",
						},
						"/spec/template/spec/containers/0/image",
						[]string{},
					},
				},
				"ephemeralContainers": {
					"ephemeral": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "tag",
							Digest:           "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
							Reference:        "fictional.registry.example:10443/imagename@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:tag",
						},
						"/spec/template/spec/ephemeralContainers/0/image",
						[]string{},
					},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "batch/v1beta1","kind": "CronJob","metadata": {"name": "hello"},"spec": {"schedule": "*/1 * * * *","jobTemplate": {"spec": {"template": {"spec": {"containers": [{"name": "hello","image": "test.example.com/test/my-app:v2"}]}}}}}}`),
			images: map[string]map[string]ImageInfo{
				"containers": {
					"hello": {
						imageutils.ImageInfo{
							Registry:         "test.example.com",
							Name:             "my-app",
							Path:             "test/my-app",
							Tag:              "v2",
							Reference:        "test.example.com/test/my-app:v2",
							ReferenceWithTag: "test.example.com/test/my-app:v2",
						},
						"/spec/jobTemplate/spec/template/spec/containers/0/image",
						[]string{},
					},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"myapp"},"spec":{"selector":{"matchLabels":{"app":"myapp"}},"template":{"metadata":{"labels":{"app":"myapp"}},"spec":{"imagePullSecrets":[{"name":"first-secret"},{"name":"second-secret"}],"initContainers":[{"name":"init","image":"fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers":[{"name":"myapp","image":"fictional.registry.example:10443/imagename"}],"ephemeralContainers":[{"name":"ephemeral","image":"fictional.registry.example:10443/imagename:tag@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}]}}}}`),
			images: map[string]map[string]ImageInfo{
				"initContainers": {
					"init": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "tag",
							Digest:           "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
							Reference:        "fictional.registry.example:10443/imagename@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:tag",
						},
						"/spec/template/spec/initContainers/0/image",
						[]string{"first-secret", "second-secret"},
					},
				},
				"containers": {
					"myapp": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "latest",
							Reference:        "fictional.registry.example:10443/imagename:latest",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:latest",
						},
						"/spec/template/spec/containers/0/image",
						[]string{"first-secret", "second-secret"},
					},
				},
				"ephemeralContainers": {
					"ephemeral": {
						imageutils.ImageInfo{
							Registry:         "fictional.registry.example:10443",
							Name:             "imagename",
							Path:             "imagename",
							Tag:              "tag",
							Digest:           "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
							Reference:        "fictional.registry.example:10443/imagename@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
							ReferenceWithTag: "fictional.registry.example:10443/imagename:tag",
						},
						"/spec/template/spec/ephemeralContainers/0/image",
						[]string{"first-secret", "second-secret"},
					},
				},
			},
		},
	}
	for _, test := range tests {
		resource, err := kubeutils.BytesToUnstructured(test.raw)
		assert.NilError(t, err)
		images, err := ExtractImagesFromResource(*resource, cfg)
		assert.NilError(t, err)
		assert.DeepEqual(t, test.images, images)
	}
}

func Test_extractImageInfo_InvalidReference(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"myapp"},"spec":{"containers":[{"name":"nginx","image":"!!invalid!!"}]}}`)
	resource, err := kubeutils.BytesToUnstructured(raw)
	assert.NilError(t, err)

	images, err := ExtractImagesFromResource(*resource, cfg)
	assert.ErrorContains(t, err, "invalid image")
	assert.Assert(t, images == nil)
}

func Test_extractImageInfo_UnregisteredKind(t *testing.T) {
	// No extractor is registered for this kind, so lookupImageExtractor returns nil
	// and the result is empty rather than an error.
	raw := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"myapp"},"data":{"image":"nginx:latest"}}`)
	resource, err := kubeutils.BytesToUnstructured(raw)
	assert.NilError(t, err)

	images, err := ExtractImagesFromResource(*resource, cfg)
	assert.NilError(t, err)
	assert.Equal(t, len(images), 0)
}
