package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestRedactSecret_WithData(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "test-secret",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"password": "c2VjcmV0cGFzc3dvcmQ=", // base64 encoded
				"apiKey":   "YXBpa2V5MTIz",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	data, found, err := unstructured.NestedMap(result.Object, "data")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "**REDACTED**", data["password"])
	assert.Equal(t, "**REDACTED**", data["apiKey"])
}

func TestRedactSecret_WithAnnotations(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "test-secret",
				"namespace": "default",
				"annotations": map[string]interface{}{
					"kubectl.kubernetes.io/last-applied-configuration": "sensitive-config",
					"custom-annotation": "sensitive-value",
				},
			},
			"data": map[string]interface{}{
				"token": "dG9rZW4xMjM=",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	metadata, found, err := unstructured.NestedMap(result.Object, "metadata")
	require.NoError(t, err)
	assert.True(t, found)

	annotations := metadata["annotations"].(map[string]interface{})
	assert.Equal(t, "**REDACTED**", annotations["kubectl.kubernetes.io/last-applied-configuration"])
	assert.Equal(t, "**REDACTED**", annotations["custom-annotation"])
}

func TestRedactSecret_EmptyData(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "empty-secret",
				"namespace": "default",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)
	assert.Equal(t, "empty-secret", result.GetName())
}

func TestRedactSecret_MultipleDataKeys(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "multi-key-secret",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key1": "dmFsdWUx",
				"key2": "dmFsdWUy",
				"key3": "dmFsdWUz",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	data, found, err := unstructured.NestedMap(result.Object, "data")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Len(t, data, 3)
	for key := range data {
		assert.Equal(t, "**REDACTED**", data[key], "key %s should be redacted", key)
	}
}

func TestRedactSecret_PreservesMetadata(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "test-secret",
				"namespace": "kube-system",
				"labels": map[string]interface{}{
					"app": "myapp",
				},
			},
			"data": map[string]interface{}{
				"secret": "c2VjcmV0",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	assert.Equal(t, "test-secret", result.GetName())
	assert.Equal(t, "kube-system", result.GetNamespace())
	labels := result.GetLabels()
	assert.Equal(t, "myapp", labels["app"])
}

func TestRedactSecret_TLSSecret(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "tls-secret",
				"namespace": "default",
			},
			"type": "kubernetes.io/tls",
			"data": map[string]interface{}{
				"tls.crt": "Y2VydGlmaWNhdGVkYXRh", // valid base64
				"tls.key": "cHJpdmF0ZWtleWRhdGE=", // valid base64
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	data, found, err := unstructured.NestedMap(result.Object, "data")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "**REDACTED**", data["tls.crt"])
	assert.Equal(t, "**REDACTED**", data["tls.key"])
}

func TestRedactSecret_DockerConfigSecret(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      "docker-secret",
				"namespace": "default",
			},
			"type": "kubernetes.io/dockerconfigjson",
			"data": map[string]interface{}{
				".dockerconfigjson": "eyJhdXRocyI6e319",
			},
		},
	}

	result, err := RedactSecret(resource)
	require.NoError(t, err)

	data, found, err := unstructured.NestedMap(result.Object, "data")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "**REDACTED**", data[".dockerconfigjson"])
}
