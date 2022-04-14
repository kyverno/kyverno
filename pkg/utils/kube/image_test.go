package kube

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/utils/image"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	"gotest.tools/assert"
)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		extractionConfig ImageExtractorConfigs
		raw              []byte
		images           map[string]map[string]imageutils.ImageInfo
	}{
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}], "ephemeralContainers": [{"name": "ephemeral", "image":"test/nginx:latest"}]}}`),
			images: map[string]map[string]image.ImageInfo{
				"initContainers": {
					"init": {Registry: "index.docker.io", Name: "busybox", Path: "busybox", Tag: "v1.2.3", Pointer: "/spec/initContainers/0/image"},
				},
				"containers": {
					"nginx": {Registry: "docker.io", Name: "nginx", Path: "nginx", Tag: "latest", Pointer: "/spec/containers/0/image"},
				},
				"ephemeralContainers": {
					"ephemeral": {Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest", Pointer: "/spec/ephemeralContainers/0/image"},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"containers": [{"name": "nginx","image": "test/nginx:latest"}]}}`),
			images: map[string]map[string]imageutils.ImageInfo{
				"containers": {
					"nginx": {Registry: "docker.io", Name: "nginx", Path: "test/nginx", Tag: "latest", Pointer: "/spec/containers/0/image"},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "apps/v1","kind": "Deployment","metadata": {"name": "myapp"},"spec": {"selector": {"matchLabels": {"app": "myapp"}},"template": {"metadata": {"labels": {"app": "myapp"}},"spec": {"initContainers": [{"name": "init","image": "fictional.registry.example:10443/imagename:tag@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"}],"containers": [{"name": "myapp","image": "fictional.registry.example:10443/imagename"}],"ephemeralContainers": [{"name": "ephemeral","image": "fictional.registry.example:10443/imagename:tag@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"}] }}}}`),
			images: map[string]map[string]imageutils.ImageInfo{
				"initContainers": {
					"init": {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", Digest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", Pointer: "/spec/template/spec/initContainers/0/image"},
				},
				"containers": {
					"myapp": {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "latest", Pointer: "/spec/template/spec/containers/0/image"},
				},
				"ephemeralContainers": {
					"ephemeral": {Registry: "fictional.registry.example:10443", Name: "imagename", Path: "imagename", Tag: "tag", Digest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", Pointer: "/spec/template/spec/ephemeralContainers/0/image"},
				},
			},
		},
		{
			raw: []byte(`{"apiVersion": "batch/v1beta1","kind": "CronJob","metadata": {"name": "hello"},"spec": {"schedule": "*/1 * * * *","jobTemplate": {"spec": {"template": {"spec": {"containers": [{"name": "hello","image": "test.example.com/test/my-app:v2"}]}}}}}}`),
			images: map[string]map[string]imageutils.ImageInfo{
				"containers": {
					"hello": {Registry: "test.example.com", Name: "my-app", Path: "test/my-app", Tag: "v2", Pointer: "/spec/jobTemplate/spec/template/spec/containers/0/image"},
				},
			},
		},
		{
			extractionConfig: ImageExtractorConfigs{
				"Task": []*ImageExtractorConfig{
					{Path: "/spec/steps/*/image"},
				},
			},
			raw: []byte(`{"apiVersion":"tekton.dev/v1beta1","kind":"Task","metadata":{"name":"example-task-name"},"spec":{"params":[{"name":"pathToDockerFile","type":"string","description":"The path to the dockerfile to build","default":"/workspace/workspace/Dockerfile"}],"resources":{"inputs":[{"name":"workspace","type":"git"}],"outputs":[{"name":"builtImage","type":"image"}]},"steps":[{"name":"ubuntu-example","image":"ubuntu","args":["ubuntu-build-example","SECRETS-example.md"]},{"image":"gcr.io/example-builders/build-example","command":["echo"],"args":["$(params.pathToDockerFile)"]},{"name":"dockerfile-pushexample","image":"gcr.io/example-builders/push-example","args":["push","$(resources.outputs.builtImage.url)"],"volumeMounts":[{"name":"docker-socket-example","mountPath":"/var/run/docker.sock"}]}],"volumes":[{"name":"example-volume","emptyDir":{}}]}}`),
			images: map[string]map[string]imageutils.ImageInfo{
				"custom": {
					"/spec/steps/0/image": {
						Registry: "docker.io",
						Name:     "ubuntu",
						Path:     "ubuntu",
						Tag:      "latest",
						Pointer:  "/spec/steps/0/image",
					},
					"/spec/steps/1/image": {
						Registry: "gcr.io",
						Name:     "build-example",
						Path:     "example-builders/build-example",
						Tag:      "latest",
						Pointer:  "/spec/steps/1/image",
					},
					"/spec/steps/2/image": {
						Registry: "gcr.io",
						Name:     "push-example",
						Path:     "example-builders/push-example",
						Tag:      "latest",
						Pointer:  "/spec/steps/2/image",
					},
				},
			},
		},
		{
			extractionConfig: ImageExtractorConfigs{
				"Task": []*ImageExtractorConfig{
					{Name: "steps", Path: "/spec/steps/*", Value: "image", Key: "name"},
				},
			}, raw: []byte(`{"apiVersion":"tekton.dev/v1beta1","kind":"Task","metadata":{"name":"example-task-name"},"spec":{"steps":[{"name":"ubuntu-example","image":"ubuntu","args":["ubuntu-build-example","SECRETS-example.md"]},{"name":"dockerfile-pushexample","image":"gcr.io/example-builders/push-example","args":["push","$(resources.outputs.builtImage.url)"]}]}}`),
			images: map[string]map[string]imageutils.ImageInfo{
				"steps": {
					"dockerfile-pushexample": {
						Registry: "gcr.io",
						Name:     "push-example",
						Path:     "example-builders/push-example",
						Tag:      "latest",
						Pointer:  "/spec/steps/1/image",
					},
					"ubuntu-example": {
						Registry: "docker.io",
						Name:     "ubuntu",
						Path:     "ubuntu",
						Tag:      "latest",
						Pointer:  "/spec/steps/0/image",
					},
				},
			},
		},
	}

	for _, test := range tests {
		resource, err := utils.ConvertToUnstructured(test.raw)
		assert.NilError(t, err)
		images, err := ExtractImagesFromResource(*resource, test.extractionConfig)
		assert.DeepEqual(t, test.images, images)
	}
}
