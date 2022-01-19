package context

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		raw                 []byte
		containers          []*ContainerImage
		initContainers      []*ContainerImage
		ephemeralContainers []*ContainerImage
	}{
		{
			raw:                 []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}], "ephemeralContainers": [{"name": "ephemeral", "image":"test/nginx:latest"}]}}`),
			initContainers:      []*ContainerImage{{Name: "init", Image: &ImageInfo{Registry: "index.docker.io", Name: "busybox", Path: "busybox", Tag: "v1.2.3", JSONPointer: "/spec/initContainers/0/image"}}},
			containers:          []*ContainerImage{{Name: "nginx", Image: &ImageInfo{Registry: "docker.io", Name: "nginx", Path: "nginx", Tag: "latest", JSONPointer: "/spec/containers/0/image"}}},
			ephemeralContainers: []*ContainerImage{{Name: "ephemeral", Image: &ImageInfo{Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest", JSONPointer: "/spec/ephemeralContainers/0/image"}}},
		},
		{
			raw:                 []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"containers": [{"name": "nginx","image": "test/nginx:latest"}]}}`),
			initContainers:      []*ContainerImage{},
			containers:          []*ContainerImage{{Name: "nginx", Image: &ImageInfo{Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest", JSONPointer: "/spec/containers/0/image"}}},
			ephemeralContainers: []*ContainerImage{},
		},
		{
			raw:                 []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "myapp"},"spec": {"selector": {"matchLabels": {"app": "myapp"}},"template": {"metadata": {"labels": {"app": "myapp"}},"spec": {"initContainers": [{"name": "init","image": "fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers": [{"name": "myapp","image": "fictional.registry.example:10443/imagename"}],"ephemeralContainers": [{"name": "ephemeral","image": "fictional.registry.example:10443/imagename:tag@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}] }}}}`),
			initContainers:      []*ContainerImage{{Name: "init", Image: &ImageInfo{Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", JSONPointer: "/spec/template/spec/initContainers/0/image", Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}}},
			containers:          []*ContainerImage{{Name: "myapp", Image: &ImageInfo{Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "latest", JSONPointer: "/spec/template/spec/containers/0/image"}}},
			ephemeralContainers: []*ContainerImage{{Name: "ephemeral", Image: &ImageInfo{Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", JSONPointer: "/spec/template/spec/ephemeralContainers/0/image", Digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}}},
		},
		{
			raw:        []byte(`{"apiVersion": "batch/v1beta1","kind": "CronJob","metadata": {"name": "hello"},"spec": {"schedule": "*/1 * * * *","jobTemplate": {"spec": {"template": {"spec": {"containers": [{"name": "hello","image": "test.example.com/test/my-app:v2"}]}}}}}}`),
			containers: []*ContainerImage{{Name: "hello", Image: &ImageInfo{Registry: "test.example.com", Name: "my-app", Path: "test/my-app", Tag: "v2", JSONPointer: "/spec/jobTemplate/spec/template/spec/containers/0/image"}}},
		},
	}

	for _, test := range tests {
		resource, err := utils.ConvertToUnstructured(test.raw)
		assert.Nil(t, err)

		init, container, ephemeral := extractImageInfo(resource, log.Log.WithName("TestExtractImageInfo"))
		if len(test.initContainers) > 0 {
			assert.Equal(t, test.initContainers, init, "unexpected initContainers %s", resource.GetName())
		}

		if len(test.containers) > 0 {
			assert.Equal(t, test.containers, container, "unexpected containers %s", resource.GetName())
		}

		if len(test.ephemeralContainers) > 0 {
			assert.Equal(t, test.ephemeralContainers, ephemeral, "unexpected ephemeralContainers %s", resource.GetName())
		}
	}
}

func Test_ImageInfo_String(t *testing.T) {
	validateImageInfo(t,
		"registry.test.io/test/myapp:v1.2-21.g5523e95@sha256:31aaf12480bd08c54e7990c6b0e43d775a7a84603d2921a6de4abbc317b2fd10",
		"myapp",
		"test/myapp",
		"registry.test.io",
		"v1.2-21.g5523e95",
		"sha256:31aaf12480bd08c54e7990c6b0e43d775a7a84603d2921a6de4abbc317b2fd10",
		"registry.test.io/test/myapp:v1.2-21.g5523e95@sha256:31aaf12480bd08c54e7990c6b0e43d775a7a84603d2921a6de4abbc317b2fd10")

	validateImageInfo(t,
		"nginx",
		"nginx",
		"nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/nginx:latest")

	validateImageInfo(t,
		"nginx:v10.3",
		"nginx",
		"nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/nginx:v10.3")

	validateImageInfo(t,
		"docker.io/test/nginx:v10.3",
		"nginx",
		"test/nginx",
		"docker.io",
		"v10.3",
		"",
		"docker.io/test/nginx:v10.3")

	validateImageInfo(t,
		"test/nginx",
		"nginx",
		"test/nginx",
		"docker.io",
		"latest",
		"",
		"docker.io/test/nginx:latest")

	validateImageInfo(t,
		"localhost:4443/test/nginx",
		"nginx",
		"test/nginx",
		"localhost:4443",
		"latest",
		"",
		"localhost:4443/test/nginx:latest")
}

func validateImageInfo(t *testing.T, raw, name, path, registry, tag, digest, str string) {
	i1, err := newImageInfo(raw, "/spec/containers/0/image")
	assert.Nil(t, err)
	assert.Equal(t, name, i1.Name)
	assert.Equal(t, path, i1.Path)
	assert.Equal(t, registry, i1.Registry)
	assert.Equal(t, tag, i1.Tag)
	assert.Equal(t, digest, i1.Digest)
	assert.Equal(t, str, i1.String())
}
