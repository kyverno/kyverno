package api

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	imageutils "github.com/kyverno/kyverno/pkg/utils/image"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
)

var cfg = config.NewDefaultConfiguration(false)

func Test_extractImageInfo(t *testing.T) {
	tests := []struct {
		extractionConfig kyvernov1.ImageExtractorConfigs
		raw              []byte
		images           map[string]map[string]ImageInfo
	}{
		{
			raw: []byte(`{"apiVersion": "v1","kind": "Pod","metadata": {"name": "myapp"},"spec": {"initContainers": [{"name": "init","image": "index.docker.io/busybox:v1.2.3"}],"containers": [{"name": "nginx","image": "nginx:latest"}], "ephemeralContainers": [{"name": "ephemeral", "image":"test/nginx:latest"}]}}`),
			images: map[string]map[string]ImageInfo{
				"initContainers": {
					"init": {
						imageutils.ImageInfo{
							Registry: "index.docker.io",
							Name:     "busybox",
							Path:     "busybox",
							Tag:      "v1.2.3",
						},
						"/spec/initContainers/0/image",
					},
				},
				"containers": {
					"nginx": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "nginx",
							Path:     "nginx",
							Tag:      "latest",
						},
						"/spec/containers/0/image",
					},
				},
				"ephemeralContainers": {
					"ephemeral": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "nginx",
							Path:     "test/nginx",
							Tag:      "latest",
						},
						"/spec/ephemeralContainers/0/image",
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
							Registry: "docker.io",
							Name:     "nginx",
							Path:     "test/nginx",
							Tag:      "latest",
						},
						"/spec/containers/0/image",
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
							Registry: "fictional.registry.example:10443",
							Name:     "imagename",
							Path:     "imagename",
							Tag:      "tag",
							Digest:   "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
						},
						"/spec/template/spec/initContainers/0/image",
					},
				},
				"containers": {
					"myapp": {
						imageutils.ImageInfo{
							Registry: "fictional.registry.example:10443",
							Name:     "imagename",
							Path:     "imagename",
							Tag:      "latest",
						},
						"/spec/template/spec/containers/0/image",
					},
				},
				"ephemeralContainers": {
					"ephemeral": {
						imageutils.ImageInfo{
							Registry: "fictional.registry.example:10443",
							Name:     "imagename",
							Path:     "imagename",
							Tag:      "tag",
							Digest:   "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
						},
						"/spec/template/spec/ephemeralContainers/0/image",
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
							Registry: "test.example.com",
							Name:     "my-app",
							Path:     "test/my-app",
							Tag:      "v2",
						},
						"/spec/jobTemplate/spec/template/spec/containers/0/image",
					},
				},
			},
		},
		{
			extractionConfig: kyvernov1.ImageExtractorConfigs{
				"Task": []kyvernov1.ImageExtractorConfig{
					{Path: "/spec/steps/*/image"},
				},
			},
			raw: []byte(`{"apiVersion":"tekton.dev/v1beta1","kind":"Task","metadata":{"name":"example-task-name"},"spec":{"params":[{"name":"pathToDockerFile","type":"string","description":"The path to the dockerfile to build","default":"/workspace/workspace/Dockerfile"}],"resources":{"inputs":[{"name":"workspace","type":"git"}],"outputs":[{"name":"builtImage","type":"image"}]},"steps":[{"name":"ubuntu-example","image":"ubuntu","args":["ubuntu-build-example","SECRETS-example.md"]},{"image":"gcr.io/example-builders/build-example","command":["echo"],"args":["$(params.pathToDockerFile)"]},{"name":"dockerfile-pushexample","image":"gcr.io/example-builders/push-example","args":["push","$(resources.outputs.builtImage.url)"],"volumeMounts":[{"name":"docker-socket-example","mountPath":"/var/run/docker.sock"}]}],"volumes":[{"name":"example-volume","emptyDir":{}}]}}`),
			images: map[string]map[string]ImageInfo{
				"custom": {
					"/spec/steps/0/image": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "ubuntu",
							Path:     "ubuntu",
							Tag:      "latest",
						},
						"/spec/steps/0/image",
					},
					"/spec/steps/1/image": {
						imageutils.ImageInfo{
							Registry: "gcr.io",
							Name:     "build-example",
							Path:     "example-builders/build-example",
							Tag:      "latest",
						},
						"/spec/steps/1/image",
					},
					"/spec/steps/2/image": {
						imageutils.ImageInfo{
							Registry: "gcr.io",
							Name:     "push-example",
							Path:     "example-builders/push-example",
							Tag:      "latest",
						},
						"/spec/steps/2/image",
					},
				},
			},
		},
		{
			extractionConfig: kyvernov1.ImageExtractorConfigs{
				"Task": []kyvernov1.ImageExtractorConfig{
					{Name: "steps", Path: "/spec/steps/*", Value: "image", Key: "name"},
				},
			}, raw: []byte(`{"apiVersion":"tekton.dev/v1beta1","kind":"Task","metadata":{"name":"example-task-name"},"spec":{"steps":[{"name":"ubuntu-example","image":"ubuntu","args":["ubuntu-build-example","SECRETS-example.md"]},{"name":"dockerfile-pushexample","image":"gcr.io/example-builders/push-example","args":["push","$(resources.outputs.builtImage.url)"]}]}}`),
			images: map[string]map[string]ImageInfo{
				"steps": {
					"dockerfile-pushexample": {
						imageutils.ImageInfo{
							Registry: "gcr.io",
							Name:     "push-example",
							Path:     "example-builders/push-example",
							Tag:      "latest",
						},
						"/spec/steps/1/image",
					},
					"ubuntu-example": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "ubuntu",
							Path:     "ubuntu",
							Tag:      "latest",
						},
						"/spec/steps/0/image",
					},
				},
			},
		},
		{
			extractionConfig: kyvernov1.ImageExtractorConfigs{
				"ClusterTask": []kyvernov1.ImageExtractorConfig{
					{Name: "steps", Path: "/spec/steps/*", Value: "image", Key: "name"},
				},
			},
			raw: []byte(`{"apiVersion":"tekton.dev/v1beta1","kind":"ClusterTask","metadata":{"name":"hello","resourceVersion":"5752181","uid":"395010b6-fe0e-4364-a7b4-6abb86974d54"},"spec":{"steps":[{"image":"alpine","name":"echo","resources":{},"script":"#!/bin/sh\necho \"Hello World\"\n"}]}}`),
			images: map[string]map[string]ImageInfo{
				"steps": {
					"echo": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "alpine",
							Path:     "alpine",
							Tag:      "latest",
						},
						"/spec/steps/0/image",
					},
				},
			},
		},
		{
			extractionConfig: kyvernov1.ImageExtractorConfigs{
				"DataVolume": []kyvernov1.ImageExtractorConfig{
					{Path: "/spec/source/registry/url", JMESPath: "trim_prefix(@, 'docker://')"},
				},
			},
			raw: []byte(`{"apiVersion":"cdi.kubevirt.io/v1beta1","kind":"DataVolume","metadata":{"name":"registry-image-datavolume"},"spec":{"source":{"registry":{"url":"docker://kubevirt/fedora-cloud-registry-disk-demo"}},"pvc":{"accessModes":["ReadWriteOnce"],"resources":{"requests":{"storage":"5Gi"}}}}}`),
			images: map[string]map[string]ImageInfo{
				"custom": {
					"/spec/source/registry/url": {
						imageutils.ImageInfo{
							Registry: "docker.io",
							Name:     "fedora-cloud-registry-disk-demo",
							Path:     "kubevirt/fedora-cloud-registry-disk-demo",
							Tag:      "latest",
						},
						"/spec/source/registry/url",
					},
				},
			},
		},
	}

	for _, test := range tests {
		resource, err := kubeutils.BytesToUnstructured(test.raw)
		assert.NilError(t, err)
		images, err := ExtractImagesFromResource(*resource, test.extractionConfig, cfg)
		assert.NilError(t, err)
		assert.DeepEqual(t, test.images, images)
	}
}
