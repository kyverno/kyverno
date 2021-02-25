package mutate

import (
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	assert "github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var inputBytes = []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myDeploy
spec:
  replica: 2
  template:
    metadata:
      labels:
        old-label: old-value
    spec:
      containers:
      - image: nginx
        name: nginx
`)

func TestTypeConversion(t *testing.T) {
	patchesJSON6902 := []byte(`
- op: replace
  path: /spec/template/spec/containers/0/name
  value: my-nginx
`)

	expectedPatches := [][]byte{
		[]byte(`{"op":"replace","path":"/spec/template/spec/containers/0/name","value":"my-nginx"}`),
	}

	// serialize resource
	inputJSON, err := yaml.YAMLToJSON(inputBytes)
	assert.Nil(t, err)

	var resource unstructured.Unstructured
	err = resource.UnmarshalJSON(inputJSON)
	assert.Nil(t, err)

	jsonPatches, err := yaml.YAMLToJSON(patchesJSON6902)
	assert.Nil(t, err)
	// apply patches
	resp, _ := ProcessPatchJSON6902("type-conversion", jsonPatches, resource, log.Log)
	if !assert.Equal(t, true, resp.Success) {
		t.Fatal(resp.Message)
	}

	assert.Equal(t, expectedPatches, resp.Patches,
		fmt.Sprintf("expectedPatches: %s\ngeneratedPatches: %s", string(expectedPatches[0]), string(resp.Patches[0])))
}

func TestJsonPatch(t *testing.T) {
	testCases := []struct {
		testName string
		// patches  []kyverno.Patch
		patches  string
		expected []byte
	}{
		{
			testName: "single patch",
			patches: `
- op: replace
  path: /spec/replica
  value: 5
`,
			expected: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myDeploy
spec:
  replica: 5
  template:
    metadata:
      labels:
        old-label: old-value
    spec:
      containers:
      - image: nginx
        name: nginx
`),
		},
		{
			testName: "insert to list",
			patches: `
- op: add
  path: /spec/template/spec/containers/1
  value: {"name":"new-nginx","image":"new-nginx-image"}
`,
			expected: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myDeploy
spec:
  replica: 2
  template:
    metadata:
      labels:
        old-label: old-value
    spec:
      containers:
      - image: nginx
        name: nginx
      - name: new-nginx
        image: new-nginx-image
`),
		},
		{
			testName: "replace first element in list",
			patches: `
- op: replace
  path: /spec/template/spec/containers/0
  value: {"name":"new-nginx","image":"new-nginx-image"}
`,
			expected: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myDeploy
spec:
  replica: 2
  template:
    metadata:
      labels:
        old-label: old-value
    spec:
      containers:
      - name: new-nginx
        image: new-nginx-image
`),
		},
		{
			testName: "multiple operations",
			patches: `
- op: replace
  path: /spec/template/spec/containers/0/name
  value: my-nginx
- op: add
  path: /spec/replica
  value: 999
- op: add
  path: /spec/template/spec/volumes
  value:
  - emptyDir:
      medium: Memory
    name: vault-secret
`,
			expected: []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myDeploy
spec:
  replica: 999
  template:
    metadata:
      labels:
        old-label: old-value
    spec:
      containers:
      - image: nginx
        name: my-nginx
      volumes:
      - emptyDir:
          medium: Memory
        name: vault-secret
`),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {

			expectedBytes, err := yaml.YAMLToJSON(testCase.expected)
			assert.Nil(t, err)

			inputBytes, err := yaml.YAMLToJSON(inputBytes)
			assert.Nil(t, err)

			patches, err := yaml.YAMLToJSON([]byte(testCase.patches))
			assert.Nil(t, err)

			out, err := applyPatchesWithOptions(inputBytes, patches)
			assert.Nil(t, err)

			if !assert.Equal(t, string(expectedBytes), string(out), testCase.testName) {
				t.FailNow()
			}
		})
	}
}
