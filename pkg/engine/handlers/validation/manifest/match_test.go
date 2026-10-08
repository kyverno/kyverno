package manifest

import (
	"encoding/json"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matcherObject(t *testing.T, source string) unstructured.Unstructured {
	t.Helper()
	var object map[string]interface{}
	require.NoError(t, yaml.Unmarshal([]byte(source), &object))
	return unstructured.Unstructured{Object: object}
}

func matcherJSON(t *testing.T, object unstructured.Unstructured) []byte {
	t.Helper()
	data, err := json.Marshal(object.Object)
	require.NoError(t, err)
	return data
}

const signedConfigMap = `apiVersion: v1
kind: ConfigMap
metadata:
  name: signed-config
data:
  key: signed-value
`

func TestMatchResourceDirectAndIgnoredFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		value     string
		ignored   []string
		wantMatch bool
	}{
		{name: "direct match", value: "signed-value", wantMatch: true},
		{name: "changed signed field", value: "changed-value"},
		{name: "explicit ignored field", value: "changed-value", ignored: []string{"data.key"}, wantMatch: true},
		{name: "other ignored field", value: "changed-value", ignored: []string{"metadata.annotations"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			object := matcherObject(t, signedConfigMap)
			require.NoError(t, unstructured.SetNestedField(object.Object, tc.value, "data", "key"))
			matched, diff, err := matchResourceWithManifest(object, []byte(signedConfigMap), tc.ignored, "", true, false, false)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, matched)
			if tc.wantMatch {
				assert.Nil(t, diff)
			} else {
				require.NotNil(t, diff)
				assert.Contains(t, diff.ToJson(), "data.key")
			}
		})
	}
}

func TestMatchResourceNamespace(t *testing.T) {
	t.Parallel()
	for _, signedNamespace := range []string{"", "signed-namespace"} {
		t.Run(signedNamespace, func(t *testing.T) {
			t.Parallel()
			manifest := matcherObject(t, signedConfigMap)
			if signedNamespace != "" {
				manifest.SetNamespace(signedNamespace)
			}
			object := manifest.DeepCopy()
			object.SetNamespace("admission-namespace")
			matched, diff, err := matchResourceWithManifest(*object, matcherJSON(t, manifest), nil, "", true, false, false)
			require.NoError(t, err)
			assert.Equal(t, signedNamespace == "", matched)
			if signedNamespace != "" {
				require.NotNil(t, diff)
				assert.Contains(t, diff.ToJson(), "metadata.namespace")
			}
		})
	}
}

func TestDirectMatchImageCanonicalization(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		before    string
		after     string
		wantMatch bool
	}{
		{name: "default registry", before: "nginx:1.14.2", after: "docker.io/nginx:1.14.2", wantMatch: true},
		{name: "default tag", before: "ubuntu", after: "ubuntu:latest", wantMatch: true},
		{name: "changed tag", before: "nginx:1.14.2", after: "nginx:1.14.3"},
		{name: "changed registry", before: "nginx:1.14.2", after: "other.example/nginx:1.14.2"},
		{name: "both expansions stay upstream mismatch", before: "ubuntu", after: "docker.io/ubuntu:latest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			manifest := unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "Pod",
				"metadata": map[string]interface{}{"name": "signed-pod"},
				"spec":     map[string]interface{}{"containers": []interface{}{map[string]interface{}{"name": "container", "image": tc.before}}},
			}}
			object := manifest.DeepCopy()
			object.Object["spec"].(map[string]interface{})["containers"].([]interface{})[0].(map[string]interface{})["image"] = tc.after
			matched, diff, err := directMatch(matcherJSON(t, manifest), matcherJSON(t, *object))
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, matched)
			if tc.wantMatch {
				assert.Nil(t, diff)
			} else {
				require.NotNil(t, diff)
			}
		})
	}
}

func TestMutatingResourceInclusion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		change    func(*testing.T, *unstructured.Unstructured)
		wantMatch bool
	}{
		{name: "added field", change: func(t *testing.T, object *unstructured.Unstructured) {
			t.Helper()
			require.NoError(t, unstructured.SetNestedField(object.Object, "added", "data", "injected"))
		}, wantMatch: true},
		{name: "modified signed field", change: func(t *testing.T, object *unstructured.Unstructured) {
			t.Helper()
			require.NoError(t, unstructured.SetNestedField(object.Object, "modified", "data", "key"))
		}},
		{name: "removed signed field", change: func(t *testing.T, object *unstructured.Unstructured) {
			t.Helper()
			unstructured.RemoveNestedField(object.Object, "data", "key")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			object := matcherObject(t, signedConfigMap)
			tc.change(t, &object)
			matched, diff, err := matchResourceWithManifest(object, []byte(signedConfigMap), nil, "", true, false, true)
			require.NoError(t, err)
			assert.Equal(t, tc.wantMatch, matched)
			if tc.wantMatch {
				assert.Nil(t, diff)
			} else {
				require.NotNil(t, diff)
			}
		})
	}
}

func TestInclusionMatchPreservesDryRunDiffDirection(t *testing.T) {
	t.Parallel()
	object := matcherObject(t, signedConfigMap)
	simulated := object.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(simulated.Object, "dry-run-value", "data", "key"))
	matched, diff, err := inclusionMatch([]byte(signedConfigMap), matcherJSON(t, object), matcherJSON(t, *simulated), true, false, false)
	require.NoError(t, err)
	assert.False(t, matched)
	require.NotNil(t, diff)
	require.Len(t, diff.Items, 1)
	assert.Equal(t, "data.key", diff.Items[0].Key)
	assert.Equal(t, "dry-run-value", diff.Items[0].Values["before"])
	assert.Equal(t, "signed-value", diff.Items[0].Values["after"])
}
