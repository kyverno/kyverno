package test

import (
	"context"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/processor"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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
	var fallbackCompileCount int
	var fallbackContextCount int
	hookVpolProviderCreated = func() { vpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	processor.HookFallbackVpolCompiled = func() { fallbackCompileCount++ }
	processor.HookFallbackContextCreated = func() { fallbackContextCount++ }
	defer func() {
		hookVpolProviderCreated = nil
		hookContextProviderCreated = nil
		processor.HookFallbackVpolCompiled = nil
		processor.HookFallbackContextCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 compilation and 1 context build across multiple resources
	assert.Equal(t, 1, vpolCompileCount, "VpolProvider should be constructed and compiled only once per testCase across all resources")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once per testCase across all resources")
	assert.Equal(t, 0, fallbackCompileCount, "PolicyProcessor should use the passed VpolProvider and never trigger fallback compilation")
	assert.Equal(t, 0, fallbackContextCount, "PolicyProcessor should use the passed ContextProvider and never trigger fallback context creation")

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
	var fallbackCompileCount int
	var fallbackContextCount int
	hookMpolProviderCreated = func() { mpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	processor.HookFallbackMpolCompiled = func() { fallbackCompileCount++ }
	processor.HookFallbackContextCreated = func() { fallbackContextCount++ }
	defer func() {
		hookMpolProviderCreated = nil
		hookContextProviderCreated = nil
		processor.HookFallbackMpolCompiled = nil
		processor.HookFallbackContextCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 compilation and 1 context build across multiple resources
	assert.Equal(t, 1, mpolCompileCount, "MpolProvider should be constructed and compiled only once per testCase across all resources")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once per testCase across all resources")
	assert.Equal(t, 0, fallbackCompileCount, "PolicyProcessor should use the passed MpolProvider and never trigger fallback compilation")
	assert.Equal(t, 0, fallbackContextCount, "PolicyProcessor should use the passed ContextProvider and never trigger fallback context creation")

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
  - payload3.json
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

	f5, err := fs.Create("payload3.json")
	require.NoError(t, err)
	_, err = f5.Write([]byte(`{"user": "", "role": "guest"}`))
	require.NoError(t, err)
	f5.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var vpolCompileCount int
	var contextBuildCount int
	var fallbackCompileCount int
	var fallbackContextCount int
	hookVpolProviderCreated = func() { vpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	processor.HookFallbackVpolJSONCompiled = func() { fallbackCompileCount++ }
	processor.HookFallbackContextCreated = func() { fallbackContextCount++ }
	defer func() {
		hookVpolProviderCreated = nil
		hookContextProviderCreated = nil
		processor.HookFallbackVpolJSONCompiled = nil
		processor.HookFallbackContextCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching for JSON payloads
	assert.Equal(t, 1, vpolCompileCount, "VpolJSONProvider should be compiled only once for multiple JSON payloads")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once for multiple JSON payloads")
	assert.Equal(t, 0, fallbackCompileCount, "PolicyProcessor should use the passed VpolJSONProvider and never trigger fallback compilation")
	assert.Equal(t, 0, fallbackContextCount, "PolicyProcessor should use the passed ContextProvider and never trigger fallback context creation")

	require.Contains(t, resp.Trigger, "payload1.json")
	require.Contains(t, resp.Trigger, "payload2.json")
	require.Contains(t, resp.Trigger, "payload3.json")
	require.NotEmpty(t, resp.Trigger["payload1.json"])
	require.NotEmpty(t, resp.Trigger["payload2.json"])
	require.NotEmpty(t, resp.Trigger["payload3.json"])
	assert.True(t, resp.Trigger["payload1.json"][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload2.json"][0].IsSuccessful())
	assert.False(t, resp.Trigger["payload3.json"][0].IsSuccessful())
	require.NotEmpty(t, resp.Trigger["payload3.json"][0].PolicyResponse.Rules)
	assert.Equal(t, engineapi.RuleStatusFail, resp.Trigger["payload3.json"][0].PolicyResponse.Rules[0].Status())
}

func TestRunTest_ProviderCachingGeneratingPolicy(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-gpol-caching
policies:
  - gpolicy.yaml
resources:
  - gresources.yaml
results:
  - policy: example-generate
    kind: Deployment
    resources:
      - app-1
      - app-2
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1
kind: GeneratingPolicy
metadata:
  name: example-generate
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
  generate:
    - expression: >
        generator.Apply(object.metadata.namespace, [{
          "apiVersion": dyn("v1"),
          "kind": dyn("ConfigMap"),
          "metadata": dyn({
            "name": dyn(object.metadata.name + "-config"),
            "namespace": dyn(object.metadata.namespace)
          })
        }])
`

	resourcesYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-1
  namespace: default
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
  name: app-2
  namespace: default
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

	f2, err := fs.Create("gpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("gresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var gpolCompileCount int
	var contextBuildCount int
	var fallbackCompileCount int
	var fallbackContextCount int
	hookGpolCompiled = func() { gpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	processor.HookFallbackGpolCompiled = func() { fallbackCompileCount++ }
	processor.HookFallbackContextCreated = func() { fallbackContextCount++ }
	defer func() {
		hookGpolCompiled = nil
		hookContextProviderCreated = nil
		processor.HookFallbackGpolCompiled = nil
		processor.HookFallbackContextCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 compilation and 1 context build across multiple resources
	assert.Equal(t, 1, gpolCompileCount, "GeneratingPolicies should be compiled only once per testCase across all resources")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed only once per testCase across all resources")
	assert.Equal(t, 0, fallbackCompileCount, "PolicyProcessor should use the passed GpolCompiledPolicies and never trigger fallback compilation")
	assert.Equal(t, 0, fallbackContextCount, "PolicyProcessor should use the passed ContextProvider and never trigger fallback context creation")

	key1 := "apps/v1,Deployment,default,app-1"
	key2 := "apps/v1,Deployment,default,app-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)
	require.NotEmpty(t, resp.Trigger[key1])
	require.NotEmpty(t, resp.Trigger[key2])
	assert.Equal(t, "example-generate", resp.Trigger[key1][0].Policy().GetName())
	assert.Equal(t, "example-generate", resp.Trigger[key2][0].Policy().GetName())
	assert.True(t, resp.Trigger[key1][0].IsSuccessful())
	assert.True(t, resp.Trigger[key2][0].IsSuccessful())

	require.NotEmpty(t, resp.Trigger[key1][0].PolicyResponse.Rules)
	require.NotEmpty(t, resp.Trigger[key2][0].PolicyResponse.Rules)
	gen1 := resp.Trigger[key1][0].PolicyResponse.Rules[0].GeneratedResources()
	gen2 := resp.Trigger[key2][0].PolicyResponse.Rules[0].GeneratedResources()
	require.NotEmpty(t, gen1)
	require.NotEmpty(t, gen2)
	assert.Equal(t, "app-1-config", gen1[0].GetName())
	assert.Equal(t, "app-2-config", gen2[0].GetName())
}

func TestRunTest_ProviderCachingGeneratingPolicyCompileFailure(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-gpol-compile-failure
policies:
  - gpolicy.yaml
resources:
  - gresources.yaml
results:
  - policy: invalid-generate
    kind: Deployment
    resources:
      - app-1
    result: fail
`

	policyYAML := `apiVersion: policies.kyverno.io/v1
kind: GeneratingPolicy
metadata:
  name: invalid-generate
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
  generate:
    - expression: "invalid syntax !!!"
`

	resourcesYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-1
  namespace: default
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
`

	f1, err := fs.Create("kyverno-test.yaml")
	require.NoError(t, err)
	_, err = f1.Write([]byte(kyvernoTestYAML))
	require.NoError(t, err)
	f1.Close()

	f2, err := fs.Create("gpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("gresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.ErrorContains(t, err, "failed to compile policy invalid-generate")
}

func TestRunTest_ProviderCachingGeneratingPolicyWithClusterResources(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-gpol-caching-cluster
policies:
  - gpolicy.yaml
resources:
  - gresources.yaml
clusterResources:
  - clusterresources.yaml
jsonPayloads:
  - payload.json
  - payload-2.json
results:
  - policy: example-generate
    kind: Deployment
    resources:
      - app-1
      - app-2
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1
kind: GeneratingPolicy
metadata:
  name: example-generate
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["deployments"]
  generate:
    - expression: >
        generator.Apply(object.metadata.namespace, [{
          "apiVersion": dyn("v1"),
          "kind": dyn("ConfigMap"),
          "metadata": dyn({
            "name": dyn(object.metadata.name + "-config"),
            "namespace": dyn(object.metadata.namespace)
          })
        }])
`

	resourcesYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-1
  namespace: default
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
  name: app-2
  namespace: default
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

	clusterResourcesYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: ClusterResource
metadata:
  name: clusterresources
spec:
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: default
`

	f1, err := fs.Create("kyverno-test.yaml")
	require.NoError(t, err)
	_, err = f1.Write([]byte(kyvernoTestYAML))
	require.NoError(t, err)
	f1.Close()

	f2, err := fs.Create("gpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("gresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	fc, err := fs.Create("clusterresources.yaml")
	require.NoError(t, err)
	_, err = fc.Write([]byte(clusterResourcesYAML))
	require.NoError(t, err)
	fc.Close()

	f4, err := fs.Create("payload.json")
	require.NoError(t, err)
	_, err = f4.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-pod", "namespace": "default"}}`))
	require.NoError(t, err)
	f4.Close()

	f5, err := fs.Create("payload-2.json")
	require.NoError(t, err)
	_, err = f5.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-pod-2", "namespace": "default"}}`))
	require.NoError(t, err)
	f5.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var gpolCompileCount int
	var contextBuildCount int
	var fallbackCompileCount int
	var fallbackContextCount int
	hookGpolCompiled = func() { gpolCompileCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	processor.HookFallbackGpolCompiled = func() { fallbackCompileCount++ }
	processor.HookFallbackContextCreated = func() { fallbackContextCount++ }
	defer func() {
		hookGpolCompiled = nil
		hookContextProviderCreated = nil
		processor.HookFallbackGpolCompiled = nil
		processor.HookFallbackContextCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// In mixed tests with cluster resources, exactly 2 context builds (1 cluster context, 1 isolated JSON context)
	// and 2 GeneratingPolicy compilations (1 for cluster context, 1 for JSON context).
	// Neither is re-compiled across multiple resources or multiple JSON payloads.
	assert.Equal(t, 2, gpolCompileCount, "GeneratingPolicies should be compiled once for cluster context and once for JSON context")
	assert.Equal(t, 2, contextBuildCount, "ContextProvider should be built once for cluster resources and once for JSON context")
	assert.Equal(t, 0, fallbackCompileCount, "PolicyProcessor should use the passed GpolCompiledPolicies and never trigger fallback compilation")
	assert.Equal(t, 0, fallbackContextCount, "PolicyProcessor should use the passed ContextProvider and never trigger fallback context creation")

	key1 := "apps/v1,Deployment,default,app-1"
	key2 := "apps/v1,Deployment,default,app-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)
	require.Contains(t, resp.Trigger, "payload.json")
	require.Contains(t, resp.Trigger, "payload-2.json")
	require.NotEmpty(t, resp.Trigger[key1])
	require.NotEmpty(t, resp.Trigger[key2])
	assert.Equal(t, "example-generate", resp.Trigger[key1][0].Policy().GetName())
	assert.Equal(t, "example-generate", resp.Trigger[key2][0].Policy().GetName())
	assert.True(t, resp.Trigger[key1][0].IsSuccessful())
	assert.True(t, resp.Trigger[key2][0].IsSuccessful())

	require.NotEmpty(t, resp.Trigger[key1][0].PolicyResponse.Rules)
	require.NotEmpty(t, resp.Trigger[key2][0].PolicyResponse.Rules)
	gen1 := resp.Trigger[key1][0].PolicyResponse.Rules[0].GeneratedResources()
	gen2 := resp.Trigger[key2][0].PolicyResponse.Rules[0].GeneratedResources()
	require.NotEmpty(t, gen1)
	require.NotEmpty(t, gen2)
	assert.Equal(t, "app-1-config", gen1[0].GetName())
	assert.Equal(t, "app-2-config", gen2[0].GetName())
}

func TestRunTest_ProviderCachingDeletingPolicyCompileFailure(t *testing.T) {
	tests := []struct {
		name                 string
		withClusterResources bool
	}{
		{
			name:                 "without cluster resources",
			withClusterResources: false,
		},
		{
			name:                 "with cluster resources",
			withClusterResources: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := memfs.New()

			kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-dpol-compile-failure
policies:
  - dpolicy.yaml
resources:
  - dresources.yaml
`
			if tt.withClusterResources {
				kyvernoTestYAML += `clusterResources:
  - clusterresources.yaml
jsonPayloads:
  - payload.json
`
			}
			kyvernoTestYAML += `results:
  - policy: invalid-delete
    kind: Pod
    resources:
      - test-nginx-1
    result: fail
`

			policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: invalid-delete
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
  conditions:
    - expression: "invalid syntax !!!"
      name: bad-syntax
  schedule: '*/1 * * * *'
`

			resourcesYAML := `apiVersion: v1
kind: Pod
metadata:
  name: test-nginx-1
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

			f2, err := fs.Create("dpolicy.yaml")
			require.NoError(t, err)
			_, err = f2.Write([]byte(policyYAML))
			require.NoError(t, err)
			f2.Close()

			f3, err := fs.Create("dresources.yaml")
			require.NoError(t, err)
			_, err = f3.Write([]byte(resourcesYAML))
			require.NoError(t, err)
			f3.Close()

			if tt.withClusterResources {
				clusterResourcesYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: ClusterResource
metadata:
  name: clusterresources
spec:
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: default
`
				fc, err := fs.Create("clusterresources.yaml")
				require.NoError(t, err)
				_, err = fc.Write([]byte(clusterResourcesYAML))
				require.NoError(t, err)
				fc.Close()

				f4, err := fs.Create("payload.json")
				require.NoError(t, err)
				_, err = f4.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-nginx-json", "namespace": "default"}}`))
				require.NoError(t, err)
				f4.Close()
			}

			testCases := test.LoadTest(fs, "kyverno-test.yaml")
			require.Len(t, testCases, 1)

			resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
			require.Error(t, err)
			assert.Nil(t, resp)
			assert.ErrorContains(t, err, "failed to compile policy invalid-delete")
		})
	}
}

func TestRunTest_ProviderCachingDeletingPolicy(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-dpol-caching
policies:
  - dpolicy.yaml
resources:
  - dresources.yaml
jsonPayloads:
  - payload.json
  - payload-2.json
results:
  - policy: deleting-pod-by-name
    kind: Pod
    resources:
      - test-nginx-1
      - test-nginx-2
    result: pass
  - policy: deleting-pod-by-name
    kind: Pod
    resources:
      - payload.json
      - payload-2.json
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: deleting-pod-by-name
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
  conditions:
    - expression: object.metadata.name.startsWith("test-nginx")
      name: check-pod-name
  schedule: '*/1 * * * *'
`

	resourcesYAML := `apiVersion: v1
kind: Pod
metadata:
  name: test-nginx-1
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
---
apiVersion: v1
kind: Pod
metadata:
  name: test-nginx-2
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

	f2, err := fs.Create("dpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("dresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	f4, err := fs.Create("payload.json")
	require.NoError(t, err)
	_, err = f4.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-nginx-json", "namespace": "default"}}`))
	require.NoError(t, err)
	f4.Close()

	f5, err := fs.Create("payload-2.json")
	require.NoError(t, err)
	_, err = f5.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-nginx-json-2", "namespace": "default"}}`))
	require.NoError(t, err)
	f5.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var dpolProviderCount int
	var contextBuildCount int
	hookDpolProviderCreated = func() { dpolProviderCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	defer func() {
		hookDpolProviderCreated = nil
		hookContextProviderCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Verify observable caching: exactly 1 dpolProvider across resources and all JSON payloads,
	// and 1 context build (reused across resources and all JSON payloads when clusterResources are absent).
	assert.Equal(t, 1, dpolProviderCount, "DpolProvider should be constructed and compiled only once per testCase across all resources and JSON payloads")
	assert.Equal(t, 1, contextBuildCount, "ContextProvider should be constructed once and reused across resources and all JSON payloads")

	key1 := "v1,Pod,default,test-nginx-1"
	key2 := "v1,Pod,default,test-nginx-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)
	require.Contains(t, resp.Trigger, "payload.json")
	require.Contains(t, resp.Trigger, "payload-2.json")
	require.NotEmpty(t, resp.Trigger[key1])
	require.NotEmpty(t, resp.Trigger[key2])
	require.NotEmpty(t, resp.Trigger["payload.json"])
	require.NotEmpty(t, resp.Trigger["payload-2.json"])
	assert.Equal(t, "deleting-pod-by-name", resp.Trigger[key1][0].Policy().GetName())
	assert.Equal(t, "deleting-pod-by-name", resp.Trigger[key2][0].Policy().GetName())
	assert.Equal(t, "deleting-pod-by-name", resp.Trigger["payload.json"][0].Policy().GetName())
	assert.Equal(t, "deleting-pod-by-name", resp.Trigger["payload-2.json"][0].Policy().GetName())
	assert.True(t, resp.Trigger[key1][0].IsSuccessful())
	assert.True(t, resp.Trigger[key2][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload.json"][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload-2.json"][0].IsSuccessful())
}

func TestRunTest_ProviderCachingDeletingPolicyWithClusterResources(t *testing.T) {
	fs := memfs.New()

	kyvernoTestYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: Test
metadata:
  name: test-dpol-caching-cluster
policies:
  - dpolicy.yaml
resources:
  - dresources.yaml
clusterResources:
  - clusterresources.yaml
globalContextEntries:
  - name: allowed-envs
    data:
      tier: production
    projections:
      - name: tier
        path: tier
jsonPayloads:
  - payload.json
  - payload-2.json
results:
  - policy: deleting-pod-by-name
    kind: Pod
    resources:
      - test-nginx-1
      - test-nginx-2
    result: pass
  - policy: deleting-pod-by-name
    kind: Pod
    resources:
      - payload.json
      - payload-2.json
    result: pass
`

	policyYAML := `apiVersion: policies.kyverno.io/v1beta1
kind: DeletingPolicy
metadata:
  name: deleting-pod-by-name
spec:
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        resources: ["pods"]
  conditions:
    - expression: globalContext.get("allowed-envs", "tier") == "production" && object.metadata.name.startsWith("test-nginx")
      name: check-pod-name
  schedule: '*/1 * * * *'
`

	resourcesYAML := `apiVersion: v1
kind: Pod
metadata:
  name: test-nginx-1
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
---
apiVersion: v1
kind: Pod
metadata:
  name: test-nginx-2
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

	f2, err := fs.Create("dpolicy.yaml")
	require.NoError(t, err)
	_, err = f2.Write([]byte(policyYAML))
	require.NoError(t, err)
	f2.Close()

	f3, err := fs.Create("dresources.yaml")
	require.NoError(t, err)
	_, err = f3.Write([]byte(resourcesYAML))
	require.NoError(t, err)
	f3.Close()

	clusterResourcesYAML := `apiVersion: cli.kyverno.io/v1alpha1
kind: ClusterResource
metadata:
  name: clusterresources
spec:
  resources:
    - apiVersion: v1
      kind: Namespace
      metadata:
        name: default
`
	fc, err := fs.Create("clusterresources.yaml")
	require.NoError(t, err)
	_, err = fc.Write([]byte(clusterResourcesYAML))
	require.NoError(t, err)
	fc.Close()

	f4, err := fs.Create("payload.json")
	require.NoError(t, err)
	_, err = f4.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-nginx-json", "namespace": "default"}}`))
	require.NoError(t, err)
	f4.Close()

	f5, err := fs.Create("payload-2.json")
	require.NoError(t, err)
	_, err = f5.Write([]byte(`{"apiVersion": "v1", "kind": "Pod", "metadata": {"name": "test-nginx-json-2", "namespace": "default"}}`))
	require.NoError(t, err)
	f5.Close()

	testCases := test.LoadTest(fs, "kyverno-test.yaml")
	require.Len(t, testCases, 1)

	var dpolProviderCount int
	var contextBuildCount int
	hookDpolProviderCreated = func() { dpolProviderCount++ }
	hookContextProviderCreated = func() { contextBuildCount++ }
	defer func() {
		hookDpolProviderCreated = nil
		hookContextProviderCreated = nil
	}()

	resp, err := runTest(context.TODO(), io.Discard, testCases[0], false)
	require.NoError(t, err)
	require.NotNil(t, resp)

	// In mixed tests with cluster resources, exactly 2 context builds (1 cluster context, 1 isolated JSON context)
	// and 2 dpolProvider compilations (1 for cluster context, 1 for JSON context).
	// Neither provider is rebuilt across multiple resources or multiple JSON payloads.
	assert.Equal(t, 2, dpolProviderCount, "DpolProvider should be compiled once for cluster resources and once for JSON context")
	assert.Equal(t, 2, contextBuildCount, "ContextProvider should be built once for cluster resources and once for JSON context")

	key1 := "v1,Pod,default,test-nginx-1"
	key2 := "v1,Pod,default,test-nginx-2"

	require.Contains(t, resp.Trigger, key1)
	require.Contains(t, resp.Trigger, key2)
	require.Contains(t, resp.Trigger, "payload.json")
	require.Contains(t, resp.Trigger, "payload-2.json")
	require.NotEmpty(t, resp.Trigger[key1])
	require.NotEmpty(t, resp.Trigger[key2])
	require.NotEmpty(t, resp.Trigger["payload.json"])
	require.NotEmpty(t, resp.Trigger["payload-2.json"])
	assert.True(t, resp.Trigger[key1][0].IsSuccessful())
	assert.True(t, resp.Trigger[key2][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload.json"][0].IsSuccessful())
	assert.True(t, resp.Trigger["payload-2.json"][0].IsSuccessful())
}
