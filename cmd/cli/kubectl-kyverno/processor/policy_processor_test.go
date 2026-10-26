package processor

import (
	"context"
	"errors"
	"os"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apiserver/pkg/admission"
	yaml "sigs.k8s.io/yaml"
)

// loadVpol decodes one ValidatingPolicy document (YAML or JSON) for the tests in this file.
func loadVpol(t *testing.T, raw []byte) policiesv1beta1.ValidatingPolicy {
	t.Helper()
	var vpol policiesv1beta1.ValidatingPolicy
	assert.NilError(t, yaml.Unmarshal(raw, &vpol))
	return vpol
}

func Test_resolveResource_fromValidatingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning ValidatingPolicies
	// when RESTMapper is unavailable in non-cluster CLI test mode.
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_fromMutatingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning MutatingPolicies.
	// This is the core fix: before this change resolveResource only checked
	// ValidatingPolicies, so MutatingPolicy tests would fail with
	// "failed to get resource from <Kind>".
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{mp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_notFound(t *testing.T) {
	// UnsafeGuessKindToResource always produces a plural form for standard
	// English kind names, so resolveResource never errors for those.
	// With an empty processor it should fall back to the guessed resource.
	p := &PolicyProcessor{}
	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_mutatingPolicyTakesPrecedence(t *testing.T) {
	// When vpol only has pods and mpol has deployments, resolving "Deployment"
	// should fall through to MutatingPolicy.
	mcVpol := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	mcMpol := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mcVpol
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mcMpol

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{mp},
	}

	// "Deployment" should match from MutatingPolicy since vpol only has pods
	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_fromGeneratingPolicy(t *testing.T) {
	// resolveResource should find resource names by scanning GeneratingPolicies.
	// This tests Fix 1: before this change, only ValidatingPolicies and
	// MutatingPolicies were scanned, so GeneratingPolicy tests would fail with
	// "failed to get resource from <Kind>".
	mc := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	gp := &policiesv1beta1.GeneratingPolicy{}
	gp.Spec.MatchConstraints = mc

	p := &PolicyProcessor{
		GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{gp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

func Test_resolveResource_generatingPolicyFallsThrough(t *testing.T) {
	// When vpol+mpol only have pods, resolving "Deployment" should
	// fall through to GeneratingPolicy which has deployments.
	mcPods := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	mcDeploys := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Rule: admissionregistrationv1.Rule{
						Resources: []string{"deployments"},
					},
				},
			},
		},
	}
	vp := &policiesv1beta1.ValidatingPolicy{}
	vp.Spec.MatchConstraints = mcPods
	mp := &policiesv1beta1.MutatingPolicy{}
	mp.Spec.MatchConstraints = mcPods
	gp := &policiesv1beta1.GeneratingPolicy{}
	gp.Spec.MatchConstraints = mcDeploys

	p := &PolicyProcessor{
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vp},
		MutatingPolicies:   []policiesv1beta1.MutatingPolicyLike{mp},
		GeneratingPolicies: []policiesv1beta1.GeneratingPolicyLike{gp},
	}

	got, err := p.resolveResource("Deployment")
	assert.NilError(t, err)
	assert.Equal(t, "deployments", got)
}

type spyVpolProvider struct {
	inner      vpolengine.Provider
	fetchCount int
}

func (s *spyVpolProvider) Fetch(ctx context.Context) ([]vpolengine.Policy, error) {
	s.fetchCount++
	return s.inner.Fetch(ctx)
}

type spyMpolProvider struct {
	inner      mpolengine.Provider
	fetchCount int
}

func (s *spyMpolProvider) Fetch(ctx context.Context, mutateExisting bool) []mpolengine.Policy {
	s.fetchCount++
	return s.inner.Fetch(ctx, mutateExisting)
}

func (s *spyMpolProvider) MatchesMutateExisting(ctx context.Context, attr admission.Attributes, req *admissionv1.AdmissionRequest, ns *corev1.Namespace, fn func() (map[string]any, error)) []string {
	return s.inner.MatchesMutateExisting(ctx, attr, req, ns, fn)
}

type spyContextProvider struct {
	libs.Context
	setGenerateContextCount int
}

func (s *spyContextProvider) SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache, useServerSideApply bool) {
	s.setGenerateContextCount++
	s.Context.SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID, restoreCache, useServerSideApply)
}

func Test_ApplyPoliciesOnResource_ReusesPrebuiltProviders(t *testing.T) {
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })

	t.Run("VpolProvider reuse across resources", func(t *testing.T) {
		vpolYAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-app-label
spec:
  validationActions: [Audit]
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [CREATE, UPDATE]
      resources: ["pods"]
  validations:
  - expression: "has(object.metadata.labels) && 'app' in object.metadata.labels"
    message: "app label is required"
`)
		passingPodYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: passing-pod
  namespace: default
  labels:
    app: demo
spec:
  containers:
  - name: nginx
    image: nginx
`)
		failingPodYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: failing-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx
`)

		vpol := loadVpol(t, vpolYAML)

		compiler := vpolcompiler.NewCompilerWithTrace(false)
		vpolLikes := []policiesv1beta1.ValidatingPolicyLike{&vpol}
		provider, err := vpolengine.NewProvider(compiler, vpolLikes, nil)
		assert.NilError(t, err)

		spy := &spyVpolProvider{inner: provider}
		contextProvider := libs.NewFakeContextProvider()

		testcases := []struct {
			name         string
			resourceYAML []byte
			expectedPass int
			expectedFail int
		}{
			{
				name:         "pod with app label passes",
				resourceYAML: passingPodYAML,
				expectedPass: 1,
				expectedFail: 0,
			},
			{
				name:         "pod without app label fails",
				resourceYAML: failingPodYAML,
				expectedPass: 0,
				expectedFail: 1,
			},
		}

		totalPass := 0
		totalFail := 0

		for _, tc := range testcases {
			resList, err := resource.GetUnstructuredResources(tc.resourceYAML)
			assert.NilError(t, err)

			rc := &ResultCounts{}
			p := PolicyProcessor{
				Store:              &store.Store{},
				ValidatingPolicies: vpolLikes,
				Resource:           *resList[0],
				Rc:                 rc,
				Out:                os.Stdout,
				Cluster:            false,
				ContextProvider:    contextProvider,
				VpolProvider:       spy,
			}
			resp, err := p.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Assert(t, len(resp) > 0)

			assert.Equal(t, rc.Pass, tc.expectedPass)
			assert.Equal(t, rc.Fail, tc.expectedFail)

			totalPass += rc.Pass
			totalFail += rc.Fail
		}

		assert.Equal(t, spy.fetchCount, len(testcases))
		assert.Equal(t, totalPass, 1)
		assert.Equal(t, totalFail, 1)
	})

	t.Run("VpolJSONProvider reuse across payloads", func(t *testing.T) {
		vpolJSONYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-json-message
spec:
  evaluation:
    mode: JSON
  validations:
  - expression: "object.message == 'hello'"
    message: "message must be hello"
`)
		jsonVpol := loadVpol(t, vpolJSONYAML)

		compiler := vpolcompiler.NewCompilerWithTrace(false)
		jsonVpolLikes := []policiesv1beta1.ValidatingPolicyLike{&jsonVpol}
		provider, err := vpolengine.NewProvider(compiler, jsonVpolLikes, nil)
		assert.NilError(t, err)

		spy := &spyVpolProvider{inner: provider}
		contextProvider := libs.NewFakeContextProvider()

		payloads := []struct {
			name         string
			payload      unstructured.Unstructured
			expectedPass int
			expectedFail int
		}{
			{
				name:         "message is hello passes",
				payload:      unstructured.Unstructured{Object: map[string]any{"message": "hello"}},
				expectedPass: 1,
				expectedFail: 0,
			},
			{
				name:         "message is world fails",
				payload:      unstructured.Unstructured{Object: map[string]any{"message": "world"}},
				expectedPass: 0,
				expectedFail: 1,
			},
		}

		totalPass := 0
		totalFail := 0

		for _, tc := range payloads {
			rc := &ResultCounts{}
			p := PolicyProcessor{
				Store:              &store.Store{},
				ValidatingPolicies: jsonVpolLikes,
				JsonPayload:        tc.payload,
				Rc:                 rc,
				Out:                os.Stdout,
				Cluster:            false,
				ContextProvider:    contextProvider,
				VpolJSONProvider:   spy,
			}
			resp, err := p.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Assert(t, len(resp) > 0)

			assert.Equal(t, rc.Pass, tc.expectedPass)
			assert.Equal(t, rc.Fail, tc.expectedFail)

			totalPass += rc.Pass
			totalFail += rc.Fail
		}

		assert.Equal(t, spy.fetchCount, len(payloads))
		assert.Equal(t, totalPass, 1)
		assert.Equal(t, totalFail, 1)
	})

	t.Run("MpolProvider reuse across resources", func(t *testing.T) {
		loaded, err := policy.Load(nil, "", "../../../../test/cli/test-mutating-policy/mutating-label/policy.yaml")
		assert.NilError(t, err)
		assert.Assert(t, len(loaded.MutatingPolicies) > 0)

		resBytes, err := os.ReadFile("../../../../test/cli/test-mutating-policy/mutating-label/resource.yaml")
		assert.NilError(t, err)
		resList, err := resource.GetUnstructuredResources(resBytes)
		assert.NilError(t, err)
		assert.Assert(t, len(resList) >= 2)

		contextProvider := libs.NewFakeContextProvider()
		compiler := mpolcompiler.NewCompilerWithTrace(false)
		provider, err := mpolengine.NewProvider(compiler, loaded.MutatingPolicies, nil, contextProvider)
		assert.NilError(t, err)

		spy := &spyMpolProvider{inner: provider}

		for i, res := range resList {
			rc := &ResultCounts{}
			p := PolicyProcessor{
				Store:            &store.Store{},
				MutatingPolicies: loaded.MutatingPolicies,
				Resource:         *res,
				Rc:               rc,
				Out:              os.Stdout,
				Cluster:          false,
				ContextProvider:  contextProvider,
				MpolProvider:     spy,
			}
			resp, err := p.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Assert(t, len(resp) > 0)
			if i == 0 {
				assert.Equal(t, resp[0].PatchedResource.GetLabels()["managed"], "")
			} else {
				assert.Equal(t, resp[0].PatchedResource.GetLabels()["managed"], "true")
			}
		}

		assert.Equal(t, spy.fetchCount, len(resList))
	})

	t.Run("GeneratingPolicy uses shared ContextProvider across resources", func(t *testing.T) {
		loaded, err := policy.Load(nil, "", "../../../../test/conformance/chainsaw/generating-policies/existing/basic-generate-data/policy.yaml")
		assert.NilError(t, err)
		assert.Assert(t, len(loaded.GeneratingPolicies) > 0)

		ns1 := unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "test-ns-1"}}}
		ns2 := unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "test-ns-2"}}}

		spyCtx := &spyContextProvider{Context: libs.NewFakeContextProvider()}

		for _, res := range []unstructured.Unstructured{ns1, ns2} {
			rc := &ResultCounts{}
			p := PolicyProcessor{
				Store:              &store.Store{},
				GeneratingPolicies: loaded.GeneratingPolicies,
				Resource:           res,
				Rc:                 rc,
				Out:                os.Stdout,
				Cluster:            false,
				ContextProvider:    spyCtx,
			}
			resp, err := p.ApplyPoliciesOnResource()
			assert.NilError(t, err)
			assert.Assert(t, len(resp) > 0)
		}

		assert.Equal(t, spyCtx.setGenerateContextCount, 2)
	})
}

func Test_ApplyPoliciesOnResource_RegistersSuppliedContextProvider_FallbackCompilation(t *testing.T) {
	contextPolicyYAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-global-env
spec:
  validationActions: [Audit]
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [CREATE, UPDATE]
      resources: ["pods"]
  validations:
  - expression: "globalContext.get('env-config', 'tier') == 'production'"
    message: "tier must be production"
`)
	vpol := loadVpol(t, contextPolicyYAML)

	passingPodYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx
`)

	podList, err := resource.GetUnstructuredResources(passingPodYAML)
	assert.NilError(t, err)

	// Set a stale global LibraryContext to verify fallback compilation binds the supplied one instead
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })
	staleCtx := libs.NewFakeContextProvider()
	staleCtx.AddGlobalReference("env-config", map[string]interface{}{"tier": "stale-env"})
	libs.LibraryContext = staleCtx

	suppliedCtx := libs.NewFakeContextProvider()
	suppliedCtx.AddGlobalReference("env-config", map[string]interface{}{"tier": "production"})

	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:              &store.Store{},
		ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{&vpol},
		Resource:           *podList[0],
		Rc:                 rc,
		Out:                os.Stdout,
		Cluster:            false,
		ContextProvider:    suppliedCtx,
		VpolProvider:       nil, // forces fallback compilation in ApplyPoliciesOnResource
	}
	resp, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Assert(t, len(resp) > 0)
	assert.Equal(t, rc.Pass, 1)
	assert.Equal(t, rc.Fail, 0)
}

func Test_ApplyPoliciesOnResource_MemoizedProviderErrors(t *testing.T) {
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })

	podList, err := resource.GetUnstructuredResources([]byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
`))
	assert.NilError(t, err)

	expectedErr := errors.New("precompilation failed")

	vpol := &policiesv1beta1.ValidatingPolicy{}
	vpol.SetName("dummy-vpol")

	vpolJSONYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: ValidatingPolicy
metadata:
  name: check-json-message
spec:
  evaluation:
    mode: JSON
`)
	jsonVpol := loadVpol(t, vpolJSONYAML)

	mpol := &policiesv1beta1.MutatingPolicy{}
	mpol.SetName("dummy-mpol")

	testCases := []struct {
		name      string
		processor PolicyProcessor
	}{
		{
			name: "vpol provider error",
			processor: PolicyProcessor{
				Store:              &store.Store{},
				ValidatingPolicies: []policiesv1beta1.ValidatingPolicyLike{vpol},
				Resource:           *podList[0],
				Rc:                 &ResultCounts{},
				Out:                os.Stdout,
				Cluster:            false,
				VpolProviderErr:    expectedErr,
			},
		},
		{
			name: "vpol json provider error",
			processor: PolicyProcessor{
				Store:               &store.Store{},
				ValidatingPolicies:  []policiesv1beta1.ValidatingPolicyLike{&jsonVpol},
				JsonPayload:         *podList[0],
				Rc:                  &ResultCounts{},
				Out:                 os.Stdout,
				Cluster:             false,
				VpolJSONProviderErr: expectedErr,
			},
		},
		{
			name: "mpol provider error",
			processor: PolicyProcessor{
				Store:            &store.Store{},
				MutatingPolicies: []policiesv1beta1.MutatingPolicyLike{mpol},
				Resource:         *podList[0],
				Rc:               &ResultCounts{},
				Out:              os.Stdout,
				Cluster:          false,
				MpolProviderErr:  expectedErr,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.processor.ApplyPoliciesOnResource()
			assert.Equal(t, err, expectedErr)
		})
	}
}
