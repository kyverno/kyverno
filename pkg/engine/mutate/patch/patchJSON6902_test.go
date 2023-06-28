package patch

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	expectedBytes := []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"myDeploy"},"spec":{"replica":2,"template":{"metadata":{"labels":{"old-label":"old-value"}},"spec":{"containers":[{"image":"nginx","name":"my-nginx"}]}}}}`)

	// serialize resource
	inputJSON, err := yaml.YAMLToJSON(inputBytes)
	require.NoError(t, err)

	var resource unstructured.Unstructured
	err = resource.UnmarshalJSON(inputJSON)
	require.NoError(t, err)

	jsonPatches, err := yaml.YAMLToJSON(patchesJSON6902)
	require.NoError(t, err)
	// apply patches
	resourceBytes, err := resource.MarshalJSON()
	require.NoError(t, err)
	patchedBytes, err := ProcessPatchJSON6902(logr.Discard(), jsonPatches, resourceBytes)
	require.NoError(t, err)
	require.Equal(t, string(expectedBytes), string(patchedBytes))
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

func Test_MissingPaths(t *testing.T) {
	tests := []struct {
		name            string
		resource        string
		patches         string
		expectedPatches map[string]bool
	}{
		// test
		{
			name: "add-map-to-non-exist-path",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
`,
			patches: `
- path: "/spec/nodeSelector"
  op: add
  value: {"node.kubernetes.io/role": "test"}
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/nodeSelector","value":{"node.kubernetes.io/role":"test"}}`: true,
			},
		},
		// test
		{
			name: "add-to-non-exist-array",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
`,
			patches: `
- path: "/spec/tolerations/0"
  op: add
  value: {"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/tolerations","value":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/test","operator":"Exists"}]}`: true,
			},
		},
		// test
		{
			name: "add-to-non-exist-array-2",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
`,
			patches: `
- path: "/spec/tolerations"
  op: add
  value: [{"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}]
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/tolerations","value":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/test","operator":"Exists"}]}`: true,
			},
		},
		// test
		{
			name: "add-to-non-exist-array-3",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
`,
			patches: `
- path: "/spec/tolerations/-1"
  op: add
  value: {"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/tolerations","value":[{"effect":"NoSchedule","key":"node-role.kubernetes.io/test","operator":"Exists"}]}`: true,
			},
		},
		// test
		{
			name: "add-to-exist-array",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
`,
			patches: `
- path: "/spec/tolerations"
  op: add
  value: [{"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}]
`,
			expectedPatches: map[string]bool{
				`{"op":"replace","path":"/spec/tolerations/0/effect","value":"NoSchedule"}`:                true,
				`{"op":"replace","path":"/spec/tolerations/0/key","value":"node-role.kubernetes.io/test"}`: true,
				`{"op":"remove","path":"/spec/tolerations/0/tolerationSeconds"}`:                           true,
			},
		},
		// test
		{
			name: "add-to-exist-array-2",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
`,
			patches: `
- path: "/spec/tolerations/-"
  op: add
  value: {"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/tolerations/1","value":{"effect":"NoSchedule","key":"node-role.kubernetes.io/test","operator":"Exists"}}`: true,
			},
		},
		// test
		{
			name: "add-to-exist-array-3",
			resource: `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
  tolerations:
  - effect: NoExecute
    key: node.kubernetes.io/not-ready
    operator: Exists
    tolerationSeconds: 300
  - key: "node.kubernetes.io/unreachable"
    operator: "Exists"
    effect: "NoExecute"
    tolerationSeconds: 6000
`,
			patches: `
- path: "/spec/tolerations/-1"
  op: add
  value: {"key": "node-role.kubernetes.io/test", "effect": "NoSchedule", "operator": "Exists"}
`,
			expectedPatches: map[string]bool{
				`{"op":"add","path":"/spec/tolerations/2","value":{"effect":"NoSchedule","key":"node-role.kubernetes.io/test","operator":"Exists"}}`: true,
			},
		},
	}

	for _, test := range tests {
		r, err := yaml.YAMLToJSON([]byte(test.resource))
		assert.Nil(t, err)

		patches, err := yaml.YAMLToJSON([]byte(test.patches))
		assert.Nil(t, err)

		patchedResource, err := applyPatchesWithOptions(r, patches)
		assert.Nil(t, err)

		generatedP, err := generatePatches(r, patchedResource)
		assert.Nil(t, err)

		for _, p := range generatedP {
			assert.Equal(t, test.expectedPatches[string(p.Json())], true)
		}
	}
}
