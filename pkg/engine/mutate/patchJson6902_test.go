package mutate

import (
	"testing"

	"github.com/ghodss/yaml"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	assert "github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const input = `
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
`

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

	mutateRule := kyverno.Mutation{
		PatchesJSON6902: `
- op: replace
  path: /spec/template/spec/containers/0/name
  value: my-nginx
`,
	}

	expectedPatches := [][]byte{
		[]byte(`{"path":"/spec/template/spec/containers/0/name","op":"replace","value":"my-nginx"}`),
	}

	// serialize resource
	inputJSONgo, err := yaml.YAMLToJSON(inputBytes)
	assert.Nil(t, err)

	var resource unstructured.Unstructured
	err = resource.UnmarshalJSON(inputJSONgo)
	assert.Nil(t, err)

	// apply patches
	resp, _ := ProcessPatchJSON6902("type-conversion", mutateRule, resource, log.Log)
	if !assert.Equal(t, true, resp.Success) {
		t.Fatal(resp.Message)
	}

	assert.Equal(t, expectedPatches, resp.Patches)
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
  path: /spec/template/spec/containers/0/command
  value:
  - arg1
  - arg2
  - arg3
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
      - command:
        - arg1
        - arg2
        - arg3
        image: nginx
        name: my-nginx
`),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {

			expectedBytes, err := yaml.YAMLToJSON(testCase.expected)
			assert.Nil(t, err)

			out, err := patchJSON6902(input, testCase.patches)

			if !assert.Equal(t, string(expectedBytes), string(out)) {
				t.FailNow()
			}
		})
	}
}
