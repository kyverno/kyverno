package api

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
)

func TestExtractImagePullSecrets(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	tests := []struct {
		name                   string
		raw                    []byte
		expectedPullSecrets    []string
		expectedImagesNotEmpty bool
	}{
		{
			name: "Pod with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "test-pod"},
				"spec": {
					"imagePullSecrets": [
						{"name": "registry-secret"},
						{"name": "gcr-secret"}
					],
					"containers": [
						{"name": "nginx", "image": "nginx:latest"}
					]
				}
			}`),
			expectedPullSecrets:    []string{"registry-secret", "gcr-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "Pod without imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "test-pod"},
				"spec": {
					"containers": [
						{"name": "nginx", "image": "nginx:latest"}
					]
				}
			}`),
			expectedPullSecrets:    nil,
			expectedImagesNotEmpty: true,
		},
		{
			name: "Deployment with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {"name": "test-deployment"},
				"spec": {
					"selector": {"matchLabels": {"app": "test"}},
					"template": {
						"metadata": {"labels": {"app": "test"}},
						"spec": {
							"imagePullSecrets": [
								{"name": "deployment-secret"}
							],
							"containers": [
								{"name": "app", "image": "myapp:v1"}
							]
						}
					}
				}
			}`),
			expectedPullSecrets:    []string{"deployment-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "StatefulSet with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "apps/v1",
				"kind": "StatefulSet",
				"metadata": {"name": "test-statefulset"},
				"spec": {
					"selector": {"matchLabels": {"app": "test"}},
					"serviceName": "test",
					"template": {
						"metadata": {"labels": {"app": "test"}},
						"spec": {
							"imagePullSecrets": [
								{"name": "statefulset-secret"}
							],
							"containers": [
								{"name": "app", "image": "myapp:v1"}
							]
						}
					}
				}
			}`),
			expectedPullSecrets:    []string{"statefulset-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "DaemonSet with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "apps/v1",
				"kind": "DaemonSet",
				"metadata": {"name": "test-daemonset"},
				"spec": {
					"selector": {"matchLabels": {"app": "test"}},
					"template": {
						"metadata": {"labels": {"app": "test"}},
						"spec": {
							"imagePullSecrets": [
								{"name": "daemonset-secret"}
							],
							"containers": [
								{"name": "app", "image": "myapp:v1"}
							]
						}
					}
				}
			}`),
			expectedPullSecrets:    []string{"daemonset-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "CronJob with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "batch/v1",
				"kind": "CronJob",
				"metadata": {"name": "test-cronjob"},
				"spec": {
					"schedule": "*/5 * * * *",
					"jobTemplate": {
						"spec": {
							"template": {
								"spec": {
									"imagePullSecrets": [
										{"name": "cronjob-secret"}
									],
									"containers": [
										{"name": "app", "image": "myapp:v1"}
									],
									"restartPolicy": "OnFailure"
								}
							}
						}
					}
				}
			}`),
			expectedPullSecrets:    []string{"cronjob-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "Job with imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "batch/v1",
				"kind": "Job",
				"metadata": {"name": "test-job"},
				"spec": {
					"template": {
						"spec": {
							"imagePullSecrets": [
								{"name": "job-secret"}
							],
							"containers": [
								{"name": "app", "image": "myapp:v1"}
							],
							"restartPolicy": "Never"
						}
					}
				}
			}`),
			expectedPullSecrets:    []string{"job-secret"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "Pod with multiple imagePullSecrets",
			raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "test-pod"},
				"spec": {
					"imagePullSecrets": [
						{"name": "secret-1"},
						{"name": "secret-2"},
						{"name": "secret-3"}
					],
					"containers": [
						{"name": "nginx", "image": "nginx:latest"}
					]
				}
			}`),
			expectedPullSecrets:    []string{"secret-1", "secret-2", "secret-3"},
			expectedImagesNotEmpty: true,
		},
		{
			name: "Pod with empty imagePullSecrets array",
			raw: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {"name": "test-pod"},
				"spec": {
					"imagePullSecrets": [],
					"containers": [
						{"name": "nginx", "image": "nginx:latest"}
					]
				}
			}`),
			expectedPullSecrets:    []string{},
			expectedImagesNotEmpty: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource, err := kube.BytesToUnstructured(test.raw)
			assert.NilError(t, err)

			images, err := ExtractImagesFromResource(*resource, nil, cfg)
			assert.NilError(t, err)

			// Get imagePullSecrets from first image (all images from same resource share same secrets)
			var imagePullSecrets []string
			for _, containerImages := range images {
				for _, img := range containerImages {
					imagePullSecrets = img.ImagePullSecrets
					break
				}
				if len(imagePullSecrets) > 0 {
					break
				}
			}

			// Check imagePullSecrets
			if test.expectedPullSecrets == nil {
				assert.Assert(t, len(imagePullSecrets) == 0, "expected no imagePullSecrets")
			} else {
				assert.Equal(t, len(test.expectedPullSecrets), len(imagePullSecrets), "imagePullSecrets count mismatch")
				for i, expected := range test.expectedPullSecrets {
					assert.Equal(t, expected, imagePullSecrets[i], "imagePullSecret name mismatch at index %d", i)
				}
			}

			// Check that images were also extracted
			if test.expectedImagesNotEmpty {
				assert.Assert(t, len(images) > 0, "expected images to be extracted")
			}
		})
	}
}

func TestExtractImagePullSecrets_Integration(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)

	// Test comprehensive Pod spec
	raw := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "complex-pod",
			"namespace": "production"
		},
		"spec": {
			"imagePullSecrets": [
				{"name": "prod-registry"},
				{"name": "backup-registry"}
			],
			"initContainers": [
				{"name": "init", "image": "init:latest"}
			],
			"containers": [
				{"name": "app", "image": "myapp:v1"},
				{"name": "sidecar", "image": "sidecar:v2"}
			],
			"ephemeralContainers": [
				{"name": "debug", "image": "debug:latest"}
			]
		}
	}`)

	resource, err := kube.BytesToUnstructured(raw)
	assert.NilError(t, err)

	images, err := ExtractImagesFromResource(*resource, nil, cfg)
	assert.NilError(t, err)

	// Get imagePullSecrets from first image
	var imagePullSecrets []string
	for _, containerImages := range images {
		for _, img := range containerImages {
			imagePullSecrets = img.ImagePullSecrets
			break
		}
		break
	}

	// Verify imagePullSecrets
	assert.Equal(t, 2, len(imagePullSecrets))
	assert.Equal(t, "prod-registry", imagePullSecrets[0])
	assert.Equal(t, "backup-registry", imagePullSecrets[1])

	// Verify all images extracted
	assert.Assert(t, len(images["initContainers"]) == 1)
	assert.Assert(t, len(images["containers"]) == 2)
	assert.Assert(t, len(images["ephemeralContainers"]) == 1)
}

func TestGetPodSpecPath(t *testing.T) {
	tests := []struct {
		kind         string
		expectedPath []string
	}{
		{"Pod", []string{"spec"}},
		{"Deployment", []string{"spec", "template", "spec"}},
		{"StatefulSet", []string{"spec", "template", "spec"}},
		{"DaemonSet", []string{"spec", "template", "spec"}},
		{"Job", []string{"spec", "template", "spec"}},
		{"ReplicaSet", []string{"spec", "template", "spec"}},
		{"ReplicationController", []string{"spec", "template", "spec"}},
		{"CronJob", []string{"spec", "jobTemplate", "spec", "template", "spec"}},
		{"Service", nil},
		{"ConfigMap", nil},
		{"Unknown", nil},
	}

	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			path := getPodSpecPath(test.kind)
			if test.expectedPath == nil {
				assert.Assert(t, path == nil, "expected nil path for %s", test.kind)
			} else {
				assert.Equal(t, len(test.expectedPath), len(path))
				for i, expected := range test.expectedPath {
					assert.Equal(t, expected, path[i])
				}
			}
		})
	}
}
