package kube

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/utils"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"gotest.tools/assert"
)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		raw    []byte
		images map[string]imageutils.ImageInfo
	}{
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}], "ephemeralContainers": [{"name": "ephemeral", "image":"test/nginx:latest"}]}}`),
			images: map[string]imageutils.ImageInfo{
				"/spec/initContainers/0/image":      {Registry: "index.docker.io", Name: "busybox", Path: "busybox", Tag: "v1.2.3"},
				"/spec/containers/0/image":          {Registry: "docker.io", Name: "nginx", Path: "nginx", Tag: "latest"},
				"/spec/ephemeralContainers/0/image": {Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest"},
			},
		},
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"containers": [{"name": "nginx","image": "test/nginx:latest"}]}}`),
			images: map[string]imageutils.ImageInfo{
				"/spec/containers/0/image": {Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest"},
			},
		},
		{
			raw: []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "myapp"},"spec": {"selector": {"matchLabels": {"app": "myapp"}},"template": {"metadata": {"labels": {"app": "myapp"}},"spec": {"initContainers": [{"name": "init","image": "fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers": [{"name": "myapp","image": "fictional.registry.example:10443/imagename"}],"ephemeralContainers": [{"name": "ephemeral","image": "fictional.registry.example:10443/imagename:tag@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}] }}}}`),
			images: map[string]imageutils.ImageInfo{
				"/spec/template/spec/initContainers/0/image":      {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"},
				"/spec/template/spec/containers/0/image":          {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "latest"},
				"/spec/template/spec/ephemeralContainers/0/image": {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", Digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},
			},
		},
		{
			raw: []byte(`{"apiVersion": "batch/v1beta1","kind": "CronJob","metadata": {"name": "hello"},"spec": {"schedule": "*/1 * * * *","jobTemplate": {"spec": {"template": {"spec": {"containers": [{"name": "hello","image": "test.example.com/test/my-app:v2"}]}}}}}}`),
			images: map[string]imageutils.ImageInfo{
				"/spec/jobTemplate/spec/template/spec/containers/0/image": {Registry: "test.example.com", Name: "my-app", Path: "test/my-app", Tag: "v2"},
			},
		},
	}

	for _, test := range tests {
		resource, err := utils.ConvertToUnstructured(test.raw)
		assert.NilError(t, err)
		images := ExtractImagesFromResource(*resource)
		// assert.Equal(t, len(test.images), len(images))
		assert.DeepEqual(t, test.images, images)
	}
}
