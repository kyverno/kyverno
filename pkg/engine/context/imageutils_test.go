package context

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		raw            []byte
		containers     []*ContainerImage
		initContainers []*ContainerImage
	}{
		{
			raw:            []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}]}}`),
			initContainers: []*ContainerImage{{Name: "init", Image: ImageInfo{Registry: "index.docker.io", Name: "busybox", Tag: "v1.2.3"}}},
			containers:     []*ContainerImage{{Name: "nginx", Image: ImageInfo{Registry: "docker.io", Name: "nginx", Tag: "latest"}}},
		},
		{
			raw:            []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "myapp"},"spec": {"selector": {"matchLabels": {"app": "myapp"}},"template": {"metadata": {"labels": {"app": "myapp"}},"spec": {"initContainers": [{"name": "init","image": "fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers": [{"name": "myapp","image": "fictional.registry.example:10443/imagename"}]}}}}`),
			initContainers: []*ContainerImage{{Name: "init", Image: ImageInfo{Registry: "fictional.registry.example:10443", Name: "imagename", Tag: "tag", Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}}},
			containers:     []*ContainerImage{{Name: "myapp", Image: ImageInfo{Registry: "fictional.registry.example:10443", Name: "imagename", Tag: "latest"}}}},
		{
			raw:        []byte(`{"apiVersion": "batch/v1beta1","kind": "CronJob","metadata": {"name": "hello"},"spec": {"schedule": "*/1 * * * *","jobTemplate": {"spec": {"template": {"spec": {"containers": [{"name": "hello","image": "b.gcr.io/test.example.com/my-app:test.example.com"}]}}}}}}`),
			containers: []*ContainerImage{{Name: "hello", Image: ImageInfo{Registry: "b.gcr.io", Name: "test.example.com/my-app", Tag: "test.example.com"}}},
		},
	}

	for _, test := range tests {
		resource, err := utils.ConvertToUnstructured(test.raw)
		assert.Nil(t, err)

		init, container := extractImageInfo(resource, log.Log.WithName("TestExtractImageInfo"))
		if len(test.initContainers) > 0 {
			assert.Equal(t, test.initContainers, init, "unexpected initContainers %s", resource.GetName())
		}

		if len(test.containers) > 0 {
			assert.Equal(t, test.containers, container, "unexpected containers %s", resource.GetName())
		}
	}
}
