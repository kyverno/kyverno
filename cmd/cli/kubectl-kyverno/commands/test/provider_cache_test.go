package test

import (
	"context"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunTest_ProviderCachingValidatingPolicy(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-provider-caching
policies:
  - policy.yaml
resources:
  - resources.yaml
results:
  - policy: check-app-label
    kind: Deployment
    resources:
      - app-deployment-1
      - app-deployment-2
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-app-label
spec:
  validationActions: [Audit]
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
  validations:
    - expression: "has(object.metadata.labels) && 'app' in object.metadata.labels"
      message: "label app is required"
`

	resourcesYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deployment-1
  namespace: default
  labels:
    app: demo1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo1
  template:
    metadata:
      labels:
        app: demo1
    spec:
      containers:
        - name: nginx
          image: nginx:latest
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deployment-2
  namespace: default
  labels:
    app: demo2
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo2
  template:
    metadata:
      labels:
        app: demo2
    spec:
      containers:
        - name: nginx
          image: nginx:latest
`

	f1, err := fs.Create("kyverno-test.yaml")
	require.NoError(t, err)
	_, err = f1.Write([]byte(kyvernoTestYAML))
	require.NoError(t, err)
	f1.Close()

	f2, err := fs.Create("policy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("resources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var vpolCompileCount int
	var contextBuildCount int
	hookVpolProviderCreated = func() { vpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	defer func() {
		hookVpolProviderCreated = nil
		hookContextProviderCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 compilation and 1 context build across multiple resources
	assert.Equal(t, 1, vpolCompileCount, "VpolProvider should be constructed and compiled only once per testCase across all resources")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once per testCase across all resources")

	key1 := "apps/v1,Deployment,default,app-deployment-1"
	key2 := "apps/v1,Deployment,default,app-deployment-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)

	require.NotEmpty(t, resp.Trigger[key1])
	require.NotEmpty(t, resp.Trigger[key2])

	assert.Equal(t, "check-app-label", resp.Trigger[key1][0].Policy().GetName())
	assert.Equal(t, "check-app-label", resp.Trigger[key2][0].Policy().GetName())
	assert.True(t, resp.Trigger[key1][0].IsSuccessful())
	assert.True(t, resp.Trigger[key2][0].IsSuccessful())
}

func TestRunTest_ProviderCachingMutatingPolicy(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-mpol-caching
policies:
  - mpolicy.yaml
resources:
  - mresources.yaml
results:
  - policy: add-label-policy
    kind: Pod
    resources:
      - pod-1
      - pod-2
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: add-label-policy
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["pods"]
  mutations:
    - patchType: ApplyConfiguration
      applyConfiguration:
        expression: >
          Object{
            metadata: Object.metadata{
              labels: Object.metadata.labels{
                cached: "true"
              }
            }
          }
`

	resourcesYAML := `apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
`

	f1, err := fs.Create("kyverno-test.yaml")
	require.NoError(t, err)
	_, err = f1.Write([]byte(kyvernoTestYAML))
	require.NoError(t, err)
	f1.Close()

	f2, err := fs.Create("mpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("mresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var mpolCompileCount int
	var contextBuildCount int
	hookMpolProviderCreated = func() { mpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	defer func() {
		hookMpolProviderCreated = nil
		hookContextProviderCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 compilation and 1 context build across multiple resources
	assert.Equal(t, 1, mpolCompileCount, "MpolProvider should be constructed and compiled only once per testCase across all resources")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once per testCase across all resources")

	key1 := "v1,Pod,default,pod-1"
	key2 := "v1,Pod,default,pod-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)

	var found1, found2 bool
	for _, r := range resp.Trigger[key1] {
		if r.PatchedResource.GetLabels()["cached"] == "true" {
			found1 = true
			assert.True(t, r.IsSuccessful())
			break
		}
	}
	for _, r := range resp.Trigger[key2] {
		if r.PatchedResource.GetLabels()["cached"] == "true" {
			found2 = true
			assert.True(t, r.IsSuccessful())
			break
		}
	}

	assert.True(t, found1, "pod-1 should be mutated with cached=true")
	assert.True(t, found2, "pod-2 should be mutated with cached=true")
}

func TestRunTest_ProviderCachingJSONPayload(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-json-payload-caching
policies:
  - json-policy.yaml
jsonPayloads:
  - payload1.json
  - payload2.json
`

	policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-json-user
spec:
  evaluation:
    mode: JSON
  validations:
    - expression: "has(object.user) && object.user != ''"
      message: "user is required"
`

	f1, err := fs.Create("kyverno-test.yaml")
	require.NoError(t, err)
	_, err = f1.Write([]byte(kyvernoTestYAML))
	require.NoError(t, err)
	f1.Close()

	f2, err := fs.Create("json-policy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("payload1.json")
	require.NoError(t, err)
	_, err = f3.Write([]byte(`{"user": "alice", "role": "admin"}`))
	require.NoError(t, err)
	f3.Close()

	f4, err := fs.Create("payload2.json")
	require.NoError(t, err)
	_, err = f4.Write([]byte(`{"user": "bob", "role": "developer"}`))
	require.NoError(t, err)
	f4.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var vpolCompileCount int
	var contextBuildCount int
	hookVpolProviderCreated = func() { vpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	defer func() {
		hookVpolProviderCreated = nil
		hookContextProviderCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching for JSON payloads
	assert.Equal(t, 1, vpolCompileCount, "VpolJSONProvider should be compiled only once for multiple JSON payloads")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once for multiple JSON payloads")

	require.Contains(t, resp.Trigger, "payload1.json")
	require.Contains(t, resp.Trigger, "payload2.json")
	require.NotEmpty(t, resp.Trigger["payload1.json"])
	require.NotEmpty(t, resp.Trigger["payload2.json"])
	assert.True(t, resp.Trigger["payload1.json"][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload2.json"][0].IsSuccessful())
}
