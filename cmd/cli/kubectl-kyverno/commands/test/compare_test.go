package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCompareJSONDoesNotInterpretKubernetesFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expected.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"kind":"document","metadata":{"creationTimestamp":"keep"},"id":9007199254740993}`), 0o600))
	actual := unstructured.Unstructured{Object: map[string]interface{}{
		"kind": "document", "metadata": map[string]interface{}{"creationTimestamp": "keep"}, "id": int64(9007199254740993),
	}}
	original := actual.DeepCopy()
	equal, _, err := getAndCompareResource(actual, nil, path, "PatchedResources", true)
	require.NoError(t, err)
	require.True(t, equal)
	require.Equal(t, original.Object, actual.Object)

	actual.Object["metadata"].(map[string]interface{})["creationTimestamp"] = "changed"
	equal, _, err = getAndCompareResource(actual, nil, path, "PatchedResources", true)
	require.NoError(t, err)
	require.False(t, equal)
}

func TestGetAndCompareResourceClusterScopedGeneratedResource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "generated-resource.yaml")

	data := []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: namespace-editor-test1-namespace-creator-sa
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: namespace-editor
subjects:
  - kind: ServiceAccount
    name: namespace-creator-sa
    namespace: test1
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	actual := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBinding",
			"metadata": map[string]interface{}{
				"name": "namespace-editor-test1-namespace-creator-sa",
			},
			"roleRef": map[string]interface{}{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind":     "ClusterRole",
				"name":     "namespace-editor",
			},
			"subjects": []interface{}{
				map[string]interface{}{
					"kind":      "ServiceAccount",
					"name":      "namespace-creator-sa",
					"namespace": "test1",
				},
			},
		},
	}

	equals, diff, err := getAndCompareResource(actual, nil, path, "GeneratedResource", false)
	if err != nil {
		t.Fatalf("expected generated resource comparison to succeed, got error: %v", err)
	}
	if !equals {
		t.Fatalf("expected generated resource to match, diff:\n%s", diff)
	}
}
