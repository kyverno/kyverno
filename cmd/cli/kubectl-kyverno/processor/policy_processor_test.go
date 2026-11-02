package processor

import (
	"context"
	"errors"
	"os"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/resource"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/variables"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
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

func Test_GetNamespaceResolver(t *testing.T) {
	localVals := &v1alpha1.ValuesSpec{
		Namespaces: []corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "local-ns",
					Labels: map[string]string{"env": "local"},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", localVals)
	assert.NilError(t, err)

	clusterNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "cluster-ns",
			Labels: map[string]string{"env": "cluster"},
		},
	}
	kclient := kubefake.NewSimpleClientset(clusterNs)
	client := dclient.NewFakeClientWithDisco(nil, kclient, nil)

	t.Run("resolves local namespace without cluster", func(t *testing.T) {
		resolver := GetNamespaceResolver(vars, nil, false)
		ns := resolver("local-ns")
		assert.Assert(t, ns != nil)
		assert.Equal(t, ns.Name, "local-ns")
		assert.Equal(t, ns.Labels["env"], "local")

		assert.Assert(t, resolver("nonexistent") == nil)
	})

	t.Run("falls back to live cluster when cluster mode is enabled and reuses cache", func(t *testing.T) {
		kclientLive := kubefake.NewSimpleClientset(clusterNs)
		clientLive := dclient.NewFakeClientWithDisco(nil, kclientLive, nil)

		resolver := GetNamespaceResolver(nil, clientLive, true)
		actionsBefore := len(kclientLive.Actions())
		ns := resolver("cluster-ns")
		assert.Assert(t, ns != nil)
		assert.Equal(t, ns.Name, "cluster-ns")
		assert.Equal(t, ns.Labels["env"], "cluster")
		assert.Equal(t, len(kclientLive.Actions()), actionsBefore+1)

		// Cached lookup should return the exact same namespace without performing a second GET
		actionsAfterFirst := len(kclientLive.Actions())
		cached := resolver("cluster-ns")
		assert.Assert(t, cached != nil)
		assert.Equal(t, cached.Name, "cluster-ns")
		assert.Equal(t, len(kclientLive.Actions()), actionsAfterFirst)

		// Missing namespace returns 404 and is cached so subsequent lookup avoids extra GET
		actionsBeforeMissing := len(kclientLive.Actions())
		assert.Assert(t, resolver("nonexistent") == nil)
		assert.Equal(t, len(kclientLive.Actions()), actionsBeforeMissing+1)

		actionsAfterMissing := len(kclientLive.Actions())
		assert.Assert(t, resolver("nonexistent") == nil)
		assert.Equal(t, len(kclientLive.Actions()), actionsAfterMissing)
	})

	t.Run("transient API errors are not cached and allow retry", func(t *testing.T) {
		kclientErr := kubefake.NewSimpleClientset()
		kclientErr.PrependReactor("get", "namespaces", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
			getAction, ok := action.(clienttesting.GetAction)
			if ok && getAction.GetName() == "transient-ns" {
				return true, nil, apierrors.NewInternalError(errors.New("transient server failure"))
			}
			return false, nil, nil
		})
		clientErr := dclient.NewFakeClientWithDisco(nil, kclientErr, nil)

		resolver := GetNamespaceResolver(nil, clientErr, true)
		assert.Assert(t, resolver("transient-ns") == nil)
		actionsCount := len(kclientErr.Actions())

		// Second call should retry the GET rather than returning a cached negative result
		assert.Assert(t, resolver("transient-ns") == nil)
		assert.Equal(t, len(kclientErr.Actions()), actionsCount+1)
	})

	t.Run("local variables take precedence over cluster namespaces", func(t *testing.T) {
		overrideVals := &v1alpha1.ValuesSpec{
			Namespaces: []corev1.Namespace{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shared-ns",
						Labels: map[string]string{"source": "local"},
					},
				},
			},
		}
		overrideVars, err := variables.New(nil, nil, "", "", overrideVals)
		assert.NilError(t, err)

		sharedClusterNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "shared-ns",
				Labels: map[string]string{"source": "cluster"},
			},
		}
		kclientShared := kubefake.NewSimpleClientset(sharedClusterNs)
		overrideClient := dclient.NewFakeClientWithDisco(nil, kclientShared, nil)

		resolver := GetNamespaceResolver(overrideVars, overrideClient, true)
		ns := resolver("shared-ns")
		assert.Assert(t, ns != nil)
		assert.Equal(t, ns.Labels["source"], "local")
	})

	t.Run("cluster lookup ignored when cluster mode is disabled", func(t *testing.T) {
		resolver := GetNamespaceResolver(nil, client, false)
		assert.Assert(t, resolver("cluster-ns") == nil)
	})

	t.Run("empty name returns nil", func(t *testing.T) {
		resolver := GetNamespaceResolver(vars, client, true)
		assert.Assert(t, resolver("") == nil)
	})

	t.Run("policy processor memoizes namespace resolver", func(t *testing.T) {
		kclientMemo := kubefake.NewSimpleClientset(clusterNs)
		clientMemo := dclient.NewFakeClientWithDisco(nil, kclientMemo, nil)
		pp := &PolicyProcessor{
			Client:  clientMemo,
			Cluster: true,
		}
		res1 := pp.getNamespaceResolver()
		assert.Assert(t, res1 != nil)
		assert.Assert(t, pp.NamespaceResolver != nil)

		actionsBefore := len(kclientMemo.Actions())
		ns1 := res1("cluster-ns")
		assert.Assert(t, ns1 != nil)
		assert.Equal(t, len(kclientMemo.Actions()), actionsBefore+1)

		res2 := pp.getNamespaceResolver()
		assert.Assert(t, res2 != nil)
		ns2 := res2("cluster-ns")
		assert.Assert(t, ns2 != nil)
		// Memoized resolver shares the exact same cache instance, issuing no additional GET
		assert.Equal(t, len(kclientMemo.Actions()), actionsBefore+1)
	})
}

func Test_ApplyPoliciesOnResource_HonorsOldObjectOnUpdate(t *testing.T) {
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })

	vpolYAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-env-change
spec:
  validationActions: [Deny]
  evaluation:
    background:
      enabled: false
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [UPDATE]
      resources: ["pods"]
  validations:
  - expression: "oldObject.metadata.labels['env'] != object.metadata.labels['env']"
    message: "env label must change"
`)
	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
  labels:
    env: prod
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

	resList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()

	t.Run("without request.oldObject on UPDATE, oldObject is object so env change validation fails", func(t *testing.T) {
		rc := &ResultCounts{}
		p := PolicyProcessor{
			Store:              &store.Store{},
			ValidatingPolicies: vpolLikes,
			Resource:           *resList[0],
			Rc:                 rc,
			Out:                os.Stdout,
			Cluster:            false,
			Operation:          "UPDATE",
			ContextProvider:    contextProvider,
			VpolProvider:       provider,
		}
		_, err := p.ApplyPoliciesOnResource()
		assert.NilError(t, err)
		assert.Equal(t, rc.Pass, 0)
		assert.Equal(t, rc.Fail, 1)
	})

	t.Run("with request.oldObject from values on UPDATE, env change validation passes", func(t *testing.T) {
		vals := &v1alpha1.ValuesSpec{
			GlobalValues: map[string]interface{}{
				"request.operation": "UPDATE",
				"request.oldObject": map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
						"labels": map[string]interface{}{
							"env": "staging",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx",
							},
						},
					},
				},
			},
		}
		vars, err := variables.New(nil, nil, "", "", vals)
		assert.NilError(t, err)
		rc := &ResultCounts{}
		p := PolicyProcessor{
			Store:              &store.Store{},
			ValidatingPolicies: vpolLikes,
			Resource:           *resList[0],
			Rc:                 rc,
			Out:                os.Stdout,
			Cluster:            false,
			Variables:          vars,
			ContextProvider:    contextProvider,
			VpolProvider:       provider,
		}
		_, err = p.ApplyPoliciesOnResource()
		assert.NilError(t, err)
		assert.Equal(t, rc.Pass, 1)
		assert.Equal(t, rc.Fail, 0)
	})
}

func Test_ApplyPoliciesOnResource_MultiplePoliciesWithDistinctOldObjects(t *testing.T) {
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })

	vpol1YAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-alpha
spec:
  validationActions: [Deny]
  evaluation:
    background:
      enabled: false
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [UPDATE]
      resources: ["pods"]
  validations:
  - expression: "oldObject.metadata.labels['env'] == 'alpha'"
    message: "old env must be alpha"
`)
	vpol2YAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-beta
spec:
  validationActions: [Deny]
  evaluation:
    background:
      enabled: false
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [UPDATE]
      resources: ["pods"]
  validations:
  - expression: "oldObject.metadata.labels['env'] == 'beta'"
    message: "old env must be beta"
`)
	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: multi-pod
  namespace: default
  labels:
    env: prod
spec:
  containers:
  - name: nginx
    image: nginx
`)

	vpol1 := loadVpol(t, vpol1YAML)
	vpol2 := loadVpol(t, vpol2YAML)
	compiler := vpolcompiler.NewCompilerWithTrace(false)
	vpolLikes := []policiesv1beta1.ValidatingPolicyLike{&vpol1, &vpol2}
	provider, err := vpolengine.NewProvider(compiler, vpolLikes, nil)
	assert.NilError(t, err)

	resList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
		Policies: []v1alpha1.Policy{
			{
				Name: "check-alpha",
				Resources: []v1alpha1.Resource{
					{
						Name: "multi-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "multi-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "alpha",
									},
								},
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "nginx",
											"image": "nginx",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "check-beta",
				Resources: []v1alpha1.Resource{
					{
						Name: "multi-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "multi-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "beta",
									},
								},
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "nginx",
											"image": "nginx",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:              &store.Store{},
		ValidatingPolicies: vpolLikes,
		Resource:           *resList[0],
		Rc:                 rc,
		Out:                os.Stdout,
		Cluster:            false,
		Variables:          vars,
		ContextProvider:    contextProvider,
		VpolProvider:       provider,
	}
	_, err = p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	// Both policies evaluate against their own distinct policy-specific oldObject and pass
	assert.Equal(t, rc.Pass, 2)
	assert.Equal(t, rc.Fail, 0)
}

func Test_ApplyPoliciesOnResource_MultipleMutatingPoliciesWithDistinctOldObjects(t *testing.T) {
	mutateAlphaYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: mutate-alpha
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  matchConditions:
  - name: check-alpha
    expression: "oldObject.metadata.labels['env'] == 'alpha'"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"mutated_alpha": "true"}}}
`)

	mutateBetaYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: mutate-beta
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  matchConditions:
  - name: check-beta
    expression: "oldObject.metadata.labels['env'] == 'beta'"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"mutated_beta": "true"}}}
`)

	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: multi-mut-pod
  namespace: default
  labels:
    app: test
spec:
  containers:
  - name: nginx
    image: nginx
`)

	var alphaPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateAlphaYAML, &alphaPol))

	var betaPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateBetaYAML, &betaPol))

	mpolLikes := []policiesv1beta1.MutatingPolicyLike{&alphaPol, &betaPol}

	resList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
		Policies: []v1alpha1.Policy{
			{
				Name: "mutate-alpha",
				Resources: []v1alpha1.Resource{
					{
						Name: "multi-mut-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "multi-mut-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "alpha",
									},
								},
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "nginx",
											"image": "nginx",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "mutate-beta",
				Resources: []v1alpha1.Resource{
					{
						Name: "multi-mut-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "multi-mut-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "beta",
									},
								},
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "nginx",
											"image": "nginx",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: mpolLikes,
		Resource:         *resList[0],
		Rc:               rc,
		Out:              os.Stdout,
		Cluster:          false,
		Variables:        vars,
		ContextProvider:  contextProvider,
	}
	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Equal(t, rc.Pass, 2)
	assert.Equal(t, rc.Fail, 0)
	assert.Assert(t, len(responses) >= 2)
	lastPatched := responses[len(responses)-1].PatchedResource
	assert.Equal(t, lastPatched.GetLabels()["mutated_alpha"], "true")
	assert.Equal(t, lastPatched.GetLabels()["mutated_beta"], "true")
}

func Test_ApplyPoliciesOnResource_MutatingInlineFallbackToOriginalOldObject(t *testing.T) {
	mutateOneYAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: mutate-one
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"mutated_one": "true"}}}
`)

	// mutate-two asserts that oldObject does NOT have mutated_one (i.e. oldObject is the pre-mutation original)
	mutateTwoYAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: mutate-two
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  matchConditions:
  - name: check-old-state-is-unmutated
    expression: "!has(oldObject.metadata.labels) || !('mutated_one' in oldObject.metadata.labels)"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"mutated_two": "true"}}}
`)

	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: chained-pod
  namespace: default
  labels:
    app: chained
spec:
  containers:
  - name: nginx
    image: nginx
`)

	var onePol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateOneYAML, &onePol))

	var twoPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateTwoYAML, &twoPol))

	mpolLikes := []policiesv1beta1.MutatingPolicyLike{&onePol, &twoPol}

	resList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: mpolLikes,
		Resource:         *resList[0],
		Rc:               rc,
		Out:              os.Stdout,
		Cluster:          false,
		Variables:        vars,
		ContextProvider:  contextProvider,
	}
	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Equal(t, rc.Pass, 2)
	assert.Equal(t, rc.Fail, 0)
	assert.Assert(t, len(responses) >= 2)
	lastPatched := responses[len(responses)-1].PatchedResource
	assert.Equal(t, lastPatched.GetLabels()["mutated_one"], "true")
	assert.Equal(t, lastPatched.GetLabels()["mutated_two"], "true")
}

func Test_ApplyPoliciesOnResource_MutateExisting_PolicySpecificOldObject(t *testing.T) {
	mutateExistingAlphaYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: mut-exist-alpha
spec:
  evaluation:
    mutateExisting:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  targetMatchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [configmaps]
  matchConditions:
  - name: check-alpha-old-object
    expression: "oldObject.metadata.labels['env'] == 'alpha'"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"alpha_applied": "true"}}}
`)

	mutateExistingBetaYAML := []byte(`
apiVersion: policies.kyverno.io/v1beta1
kind: MutatingPolicy
metadata:
  name: mut-exist-beta
spec:
  evaluation:
    mutateExisting:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  targetMatchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [configmaps]
  matchConditions:
  - name: check-beta-old-object
    expression: "oldObject.metadata.labels['env'] == 'beta'"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"beta_applied": "true"}}}
`)

	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: trigger-pod
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx
`)

	cmYAML := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: target-cm
  namespace: default
  labels:
    app: target
data:
  key: value
`)

	var alphaPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateExistingAlphaYAML, &alphaPol))

	var betaPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutateExistingBetaYAML, &betaPol))

	mpolLikes := []policiesv1beta1.MutatingPolicyLike{&alphaPol, &betaPol}

	podList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	cmList, err := resource.GetUnstructuredResources(cmYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
		Policies: []v1alpha1.Policy{
			{
				Name: "mut-exist-alpha",
				Resources: []v1alpha1.Resource{
					{
						Name: "trigger-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "trigger-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "alpha",
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "mut-exist-beta",
				Resources: []v1alpha1.Resource{
					{
						Name: "trigger-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "trigger-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "beta",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: mpolLikes,
		Resource:         *podList[0],
		TargetResources:  cmList,
		Rc:               rc,
		Out:              os.Stdout,
		Cluster:          false,
		Variables:        vars,
		ContextProvider:  contextProvider,
	}
	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Assert(t, len(responses) >= 2)
	var alphaTarget, betaTarget *unstructured.Unstructured
	for _, resp := range responses {
		for _, r := range resp.PolicyResponse.Rules {
			if pt, _, _ := r.PatchedTarget(); pt != nil {
				if pt.GetLabels()["alpha_applied"] == "true" {
					alphaTarget = pt
				}
				if pt.GetLabels()["beta_applied"] == "true" {
					betaTarget = pt
				}
			}
		}
	}
	assert.Assert(t, alphaTarget != nil)
	assert.Assert(t, betaTarget != nil)
}

func TestPolicyProcessor_MutatingPolicy_RequestObjectLoopInvariant(t *testing.T) {
	// Policy 1 mutates the pod by adding label `step: "one"`
	policy1YAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: mutate-step-one
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"step": "one"}}}
`)

	// Policy 2 asserts that:
	// 1. `request.object` is the unmutated admission object (does not have label `step: "one"`).
	// 2. `object` has the mutation from policy 1 (has label `step: "one"`).
	// 3. `request.oldObject` is policy 2's configured oldObject.
	policy2YAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: mutate-step-two
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  matchConditions:
  - name: check-request-object-and-old-object
    expression: >
      !('step' in request.object.metadata.labels) &&
      object.metadata.labels['step'] == 'one' &&
      oldObject.metadata.labels['env'] == 'two-old'
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"step_two_applied": "true"}}}
`)

	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
  namespace: default
  labels:
    app: demo
spec:
  containers:
  - name: nginx
    image: nginx
`)

	var pol1 policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(policy1YAML, &pol1))
	var pol2 policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(policy2YAML, &pol2))

	mpolLikes := []policiesv1beta1.MutatingPolicyLike{&pol1, &pol2}

	podList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
		Policies: []v1alpha1.Policy{
			{
				Name: "mutate-step-two",
				Resources: []v1alpha1.Resource{
					{
						Name: "test-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "test-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "two-old",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()
	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: mpolLikes,
		Resource:         *podList[0],
		Rc:               rc,
		Out:              os.Stdout,
		Cluster:          false,
		Variables:        vars,
		ContextProvider:  contextProvider,
	}

	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Assert(t, len(responses) >= 2)
	var finalPatched unstructured.Unstructured
	for _, resp := range responses {
		if resp.PatchedResource.Object != nil && len(resp.PolicyResponse.Rules) > 0 {
			finalPatched = resp.PatchedResource
		}
	}
	assert.Equal(t, finalPatched.GetLabels()["step"], "one")
	assert.Equal(t, finalPatched.GetLabels()["step_two_applied"], "true")
}

func TestPolicyProcessor_MutateExisting_TriggerObservesInlineMutations(t *testing.T) {
	// Inline MutatingPolicy adds label `inline_applied: "true"`
	inlinePolicyYAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: inline-mut
spec:
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"inline_applied": "true"}}}
`)

	// MutateExisting policy matches when trigger request.object has `inline_applied: "true"`
	// and oldObject has `env: "staging"`.
	mutExistYAML := []byte(`
apiVersion: policies.kyverno.io/v1alpha1
kind: MutatingPolicy
metadata:
  name: mut-exist-trigger-inline
spec:
  evaluation:
    mutateExisting:
      enabled: true
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [pods]
  targetMatchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: [v1]
      operations: [UPDATE]
      resources: [configmaps]
  matchConditions:
  - name: check-inline-mutation-on-trigger
    expression: "request.object.metadata.labels['inline_applied'] == 'true' && oldObject.metadata.labels['env'] == 'staging'"
  mutations:
  - patchType: ApplyConfiguration
    applyConfiguration:
      expression: >
        Object{metadata: Object.metadata{labels: {"target_mutated": "true"}}}
`)

	podYAML := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: trigger-pod
  namespace: default
  labels:
    app: demo
spec:
  containers:
  - name: nginx
    image: nginx
`)

	cmYAML := []byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: target-cm
  namespace: default
  labels:
    app: target
data:
  key: value
`)

	var inlinePol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(inlinePolicyYAML, &inlinePol))
	var existPol policiesv1beta1.MutatingPolicy
	assert.NilError(t, yaml.Unmarshal(mutExistYAML, &existPol))

	mpolLikes := []policiesv1beta1.MutatingPolicyLike{&inlinePol, &existPol}

	podList, err := resource.GetUnstructuredResources(podYAML)
	assert.NilError(t, err)
	cmList, err := resource.GetUnstructuredResources(cmYAML)
	assert.NilError(t, err)

	vals := &v1alpha1.ValuesSpec{
		GlobalValues: map[string]interface{}{
			"request.operation": "UPDATE",
		},
		Policies: []v1alpha1.Policy{
			{
				Name: "mut-exist-trigger-inline",
				Resources: []v1alpha1.Resource{
					{
						Name: "trigger-pod",
						Values: map[string]interface{}{
							"request.oldObject": map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "Pod",
								"metadata": map[string]interface{}{
									"name":      "trigger-pod",
									"namespace": "default",
									"labels": map[string]interface{}{
										"env": "staging",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	vars, err := variables.New(nil, nil, "", "", vals)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()
	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:            &store.Store{},
		MutatingPolicies: mpolLikes,
		Resource:         *podList[0],
		TargetResources:  cmList,
		Rc:               rc,
		Out:              os.Stdout,
		Cluster:          false,
		Variables:        vars,
		ContextProvider:  contextProvider,
	}

	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Assert(t, len(responses) >= 2)
	var targetPatched *unstructured.Unstructured
	for _, resp := range responses {
		for _, r := range resp.PolicyResponse.Rules {
			if pt, _, _ := r.PatchedTarget(); pt != nil {
				if pt.GetLabels()["target_mutated"] == "true" {
					targetPatched = pt
				}
			}
		}
	}
	assert.Assert(t, targetPatched != nil)
}

func Test_ApplyPoliciesOnResource_ValidatingPolicy_AutogenDeployment(t *testing.T) {
	prevLibraryContext := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibraryContext })

	vpolYAML := []byte(`
apiVersion: policies.kyverno.io/v1
kind: ValidatingPolicy
metadata:
  name: check-deployment-autogen
spec:
  validationActions: [Deny]
  evaluation:
    background:
      enabled: false
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: [CREATE]
      resources: ["pods"]
  validations:
  - expression: "object.spec.containers.all(c, c.image.startsWith('nginx'))"
    message: "only nginx containers allowed"
`)
	deployYAML := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`)

	vpol := loadVpol(t, vpolYAML)
	compiler := vpolcompiler.NewCompilerWithTrace(false)
	vpolLikes := []policiesv1beta1.ValidatingPolicyLike{&vpol}
	provider, err := vpolengine.NewProvider(compiler, vpolLikes, nil)
	assert.NilError(t, err)

	resList, err := resource.GetUnstructuredResources(deployYAML)
	assert.NilError(t, err)

	contextProvider := libs.NewFakeContextProvider()
	rc := &ResultCounts{}
	p := PolicyProcessor{
		Store:              &store.Store{},
		ValidatingPolicies: vpolLikes,
		Resource:           *resList[0],
		Rc:                 rc,
		Out:                os.Stdout,
		Cluster:            false,
		Operation:          "CREATE",
		ContextProvider:    contextProvider,
		VpolProvider:       provider,
	}

	responses, err := p.ApplyPoliciesOnResource()
	assert.NilError(t, err)
	assert.Assert(t, len(responses) > 0)
	assert.Equal(t, rc.Pass, 1)
	assert.Equal(t, rc.Fail, 0)
}
