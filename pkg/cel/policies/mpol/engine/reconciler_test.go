package engine

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Mocks

type fakeClient struct {
	client.Client
	policy *policiesv1beta1.MutatingPolicy
	nmpol  *policiesv1beta1.NamespacedMutatingPolicy
	err    error
}

func (f *fakeClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.err != nil {
		return f.err
	}
	switch o := obj.(type) {
	case *policiesv1beta1.MutatingPolicy:
		if f.policy != nil && key.Name == f.policy.Name && key.Namespace == f.policy.Namespace {
			*o = *f.policy
			return nil
		}
	case *policiesv1beta1.NamespacedMutatingPolicy:
		if f.nmpol != nil && key.Name == f.nmpol.Name && key.Namespace == f.nmpol.Namespace {
			*o = *f.nmpol
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, "")
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("successful reconciliation of cluster-scoped MutatingPolicy", func(t *testing.T) {
		mp := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		}
		rec := newReconciler(
			&fakeClient{policy: mp},
			compiler.NewCompiler(),
			nil, false,
		)
		// Cluster-scoped: no namespace in request.
		name := types.NamespacedName{Name: "test-policy"}
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})

	t.Run("successful reconciliation of NamespacedMutatingPolicy", func(t *testing.T) {
		nmp := &policiesv1beta1.NamespacedMutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "test-ns"},
		}
		rec := newReconciler(
			&fakeClient{nmpol: nmp},
			compiler.NewCompiler(),
			nil, false,
		)
		// Namespaced: namespace in request.
		name := types.NamespacedName{Namespace: "test-ns", Name: "test-nmpol"}
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)
	})
}

// disallowLatestTagMutatingPolicy matches Pods and autogens onto both a built-in kind (deployments)
// and a custom workload CRD named by the explicit "<resource>.<version>.<group>" format, which does
// not need a live cluster to resolve.
func disallowLatestTagMutatingPolicy() *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{spec: Object.spec{containers: object.spec.containers}}`,
				},
			}},
		},
	}
}

// TestReconcile_ExtractionMode is the regression test for the gap that let the mpol extraction wiring
// ship without actually working end to end: provider.go (used by NewProvider, the CLI/test path) set
// Policy.ExtractionMode correctly, but reconciler.go (used by the live cluster controller) had its own
// separate copy of the same loop that never set it, so the live admission path would have tried to
// apply the policy's Pod-shaped mutation directly to the real custom-CRD object instead of skipping it.
func TestReconcile_ExtractionMode(t *testing.T) {
	ctx := context.Background()
	rec := newReconciler(
		&fakeClient{policy: disallowLatestTagMutatingPolicy()},
		compiler.NewCompiler(),
		nil, false,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "add-team-label"}})
	require.NoError(t, err)

	fetched := rec.Fetch(ctx, false)
	require.Len(t, fetched, 3, "expected the base policy plus one autogen'd variant per ReplacementsRef group")

	var sawDeploymentVariant, sawJobSetVariant bool
	for _, p := range fetched {
		mc := p.Policy.GetSpec().MatchConstraints
		if len(mc.ResourceRules) == 0 {
			continue // the base, Pod-targeted policy
		}
		resources := mc.ResourceRules[0].Resources
		switch {
		case len(resources) > 0 && resources[0] == "deployments":
			sawDeploymentVariant = true
			assert.False(t, p.ExtractionMode, "the built-in deployments variant must not be extraction-mode")
		case len(resources) > 0 && resources[0] == "jobsets":
			sawJobSetVariant = true
			assert.True(t, p.ExtractionMode, "the custom-CRD jobsets variant must be extraction-mode")
		}
	}
	assert.True(t, sawDeploymentVariant, "expected a deployments autogen variant")
	assert.True(t, sawJobSetVariant, "expected a jobsets autogen variant")
}

// reconcilerTestExceptionLister is a fixed-set celengine.PolicyExceptionLister that filters through the
// real celengine.ExceptionScopeAll.Select, so it exercises actual namespace-matching logic rather than
// stubbing it out.
type reconcilerTestExceptionLister struct {
	exceptions []*policiesv1beta1.PolicyException
}

func (l *reconcilerTestExceptionLister) List(labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return l.exceptions, nil
}

func (l *reconcilerTestExceptionLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	return celengine.ExceptionScopeAll.Select(l.exceptions, kind, name, namespace), nil
}

// TestReconcile_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument passed
// to ListForPolicy: under scope "*" a namespaced policy must only accept exceptions filed in its own
// namespace, never a foreign one. It fails if Reconcile ever stops passing policy.GetNamespace()
// through to the exception lookup.
func TestReconcile_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	ctx := context.Background()
	policy := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{"CREATE"},
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}},
					},
				}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{metadata: Object.metadata{labels: {"env": "test"}}}`,
				},
			}},
		},
	}
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedMutatingPolicy"}},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedMutatingPolicy"}},
		},
	}
	lister := &reconcilerTestExceptionLister{exceptions: []*policiesv1beta1.PolicyException{ownException, foreignException}}
	rec := newReconciler(&fakeClient{nmpol: policy}, compiler.NewCompiler(), lister, true)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "tenant-a", Name: "guard"}})
	require.NoError(t, err)

	eng := NewEngine(
		rec,
		func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
		matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{},
	)
	// The request must be in tenant-a: a NamespacedMutatingPolicy only matches requests in its own
	// namespace. Handle (the admission path), not Evaluate (the mutate-existing path), is what
	// drives Fetch(ctx, false), so it is the path reconciler.go's ListForPolicy argument actually
	// serves in production.
	requestObject := `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"tenant-a"}}`
	req := celengine.EngineRequest{
		Request: admissionv1.AdmissionRequest{
			Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			Namespace: "tenant-a",
			Name:      "nginx",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(requestObject)},
		},
	}
	resp, err := eng.Handle(ctx, req, predicate)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)
	rule := resp.Policies[0].Rules[0]
	require.Len(t, rule.Exceptions(), 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", rule.Exceptions()[0].GetNamespace())
	assert.Equal(t, "own", rule.Exceptions()[0].GetName())
}

func TestReconcile_JSONModeDoesNotGenerateAutogenPolicies(t *testing.T) {
	ctx := context.Background()
	policy := jsonMutationPolicy("json-pod", `[JSONPatch{op:"add",path:"/metadata/labels/mutated",value:"yes"}]`)
	policy.Spec.MatchConstraints = &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}},
			},
		}},
	}
	rec := newReconciler(&fakeClient{policy: policy}, compiler.NewCompiler(), nil, false)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)
	require.Len(t, rec.Fetch(ctx, false), 1)
}

func TestFetch(t *testing.T) {
	trueBool := true
	falseBool := false

	tests := []struct {
		name           string
		mutateExisting bool
		policyMap      map[string][]Policy
		expectedNames  []string
	}{
		{
			name:           "no policies",
			mutateExisting: false,
			policyMap:      map[string][]Policy{},
			expectedNames:  []string{},
		},
		{
			name:           "mutateExisting = false, return all policies",
			mutateExisting: false,
			policyMap: map[string][]Policy{
				"ns1/policy1": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
						},
					},
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
						},
					},
				},
			},
			expectedNames: []string{"policy1", "policy2"},
		},
		{
			name:           "mutateExisting = true, only enabled ones returned",
			mutateExisting: true,
			policyMap: map[string][]Policy{
				"ns1/policy1": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
							},
						},
					},
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &falseBool,
									},
								},
							},
						},
					},
				},
			},
			expectedNames: []string{"policy1"},
		},
		{
			name:           "mutateExisting = true, all disabled",
			mutateExisting: true,
			policyMap: map[string][]Policy{
				"ns1/policy2": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &falseBool,
									},
								},
							},
						},
					},
				},
			},
			expectedNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &reconciler{
				policies: tt.policyMap,
				lock:     &sync.RWMutex{},
			}

			got := r.Fetch(context.Background(), tt.mutateExisting)

			var gotNames []string
			for _, p := range got {
				gotNames = append(gotNames, p.Policy.GetName())
			}
			assert.ElementsMatch(t, tt.expectedNames, gotNames)
		})
	}
}

type fakeFetchWithError struct {
	*reconciler
}

func (f *fakeFetchWithError) Fetch(ctx context.Context, mutateExisting bool) ([]Policy, error) {
	return nil, fmt.Errorf("simulated fetch error")
}

func TestMatchesMutateExisting(t *testing.T) {
	trueBool := true

	tests := []struct {
		name          string
		policies      map[string][]Policy
		expectedNames []string
	}{
		{
			name: "single policy matches with conditions true",
			policies: map[string][]Policy{
				"test/policy1": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{}, // empty constraints should match
								MatchConditions:  nil,                                       // no conditions
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{"policy1"},
		},
		{
			name: "policy with conditions that fail",
			policies: map[string][]Policy{
				"test/policy2": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy2"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
								MatchConditions: []admissionregistrationv1.MatchCondition{
									{
										Expression: `object.metadata.labels.env == "dev"`,
									},
								},
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{},
		},
		{
			name: "no mutateExisting enabled, nothing matched",
			policies: map[string][]Policy{
				"test/policy3": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy3"},
						},
					},
				},
			},
			expectedNames: []string{},
		},
		{
			name: "namespaced policy returns stable policy key",
			policies: map[string][]Policy{
				"default/policy4": {
					{
						Policy: &policiesv1beta1.NamespacedMutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy4", Namespace: "default"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{"default/policy4"},
		},
		{
			name: "admission-disabled policy is never matched by admission requests",
			policies: map[string][]Policy{
				"test/policy5": {
					{
						Policy: &policiesv1beta1.MutatingPolicy{
							ObjectMeta: metav1.ObjectMeta{Name: "policy5"},
							Spec: policiesv1beta1.MutatingPolicySpec{
								EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
									Admission: &policiesv1beta1.AdmissionConfiguration{Enabled: ptr.To(false)},
									MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
										Enabled: &trueBool,
									},
								},
								MatchConstraints: &admissionregistrationv1.MatchResources{},
							},
						},
						CompiledPolicy: &compiler.Policy{},
					},
				},
			},
			expectedNames: []string{},
		},
	}

	comp := compiler.NewCompiler()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, l := range tt.policies {
				for i, p := range l {
					c, _ := comp.Compile(p.Policy, nil)
					tt.policies[k][i].CompiledPolicy = c
				}
			}
			r := &reconciler{
				lock:     &sync.RWMutex{},
				policies: tt.policies,
			}
			attrs := &mockAttributes{}
			namespace := &corev1.Namespace{}
			got := r.MatchesMutateExisting(context.TODO(), attrs, nil, namespace, nil)
			assert.ElementsMatch(t, tt.expectedNames, got)
		})
	}
}

type mockPolexLister struct {
	exceptions []*policiesv1beta1.PolicyException
}

func (m *mockPolexLister) List(_ labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return m.exceptions, nil
}

func (m *mockPolexLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	return celengine.ExceptionScopeAll.Select(m.exceptions, kind, name, namespace), nil
}

type compileCall struct {
	policy     policiesv1beta1.MutatingPolicyLike
	exceptions []*policiesv1beta1.PolicyException
}

type captureCompiler struct {
	compiler.Compiler
	calls []compileCall
}

func (c *captureCompiler) Compile(policy policiesv1beta1.MutatingPolicyLike, exceptions []*policiesv1beta1.PolicyException) (*compiler.Policy, field.ErrorList) {
	c.calls = append(c.calls, compileCall{policy: policy, exceptions: exceptions})
	return c.Compiler.Compile(policy, exceptions)
}

func TestReconcile_AutogeneratedExceptionRewriting(t *testing.T) {
	ctx := context.Background()
	policy := disallowLatestTagMutatingPolicy()
	policy.Spec.AutogenConfiguration = &policiesv1beta1.MutatingPolicyAutogenConfiguration{
		PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
			Controllers: []string{"deployments", "cronjobs", "jobsets.v1alpha2.jobset.x-k8s.io"},
		},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt-nginx"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policy.Name,
				Kind: "MutatingPolicy",
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match-nginx",
					Expression: "object.spec.containers.exists(c, c.name == 'nginx')",
				},
			},
		},
	}

	polexLister := &mockPolexLister{exceptions: []*policiesv1beta1.PolicyException{exception}}
	capComp := &captureCompiler{Compiler: compiler.NewCompiler()}
	rec := newReconciler(
		&fakeClient{policy: policy},
		capComp,
		polexLister, true,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	sawDeploymentVariant := false
	sawCronJobVariant := false
	sawJobSetVariant := false

	for _, call := range capComp.calls {
		mc := call.policy.GetMatchConstraints()
		if len(mc.ResourceRules) == 0 {
			continue
		}
		resources := mc.ResourceRules[0].Resources
		if slices.Contains(resources, "deployments") {
			sawDeploymentVariant = true
			require.Len(t, call.exceptions, 1)
			assert.Equal(t, "object.spec.template.spec.containers.exists(c, c.name == 'nginx')", call.exceptions[0].Spec.MatchConditions[0].Expression)
		}
		if slices.Contains(resources, "cronjobs") {
			sawCronJobVariant = true
			require.Len(t, call.exceptions, 1)
			assert.Equal(t, "object.spec.jobTemplate.spec.template.spec.containers.exists(c, c.name == 'nginx')", call.exceptions[0].Spec.MatchConditions[0].Expression)
		}
		if slices.Contains(resources, "jobsets") {
			sawJobSetVariant = true
			require.Len(t, call.exceptions, 1)
			assert.Equal(t, "object.spec.containers.exists(c, c.name == 'nginx')", call.exceptions[0].Spec.MatchConditions[0].Expression, "extraction-mode target must preserve Pod-shaped exception expressions")
		}
	}

	assert.True(t, sawDeploymentVariant, "expected deployments variant compiled with rewritten exception")
	assert.True(t, sawCronJobVariant, "expected cronjobs variant compiled with rewritten exception")
	assert.True(t, sawJobSetVariant, "expected jobsets extraction-mode variant compiled with original exception")
}

func TestReconcile_AutogeneratedExceptionRuntimeEvaluation(t *testing.T) {
	ctx := context.Background()
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "set-allow-privilege-escalation"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"deployments", "cronjobs"},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: `Object{
						spec: Object.spec{
							containers: object.spec.containers.map(container, Object.spec.containers{
								name: container.name,
								securityContext: Object.spec.containers.securityContext{
									allowPrivilegeEscalation: false
								}
							})
						}
					}`,
				},
			}},
		},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt-nginx"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policy.Name,
				Kind: "MutatingPolicy",
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match-nginx",
					Expression: "object.spec.containers.exists(c, c.name == 'nginx')",
				},
			},
		},
	}

	polexLister := &mockPolexLister{exceptions: []*policiesv1beta1.PolicyException{exception}}
	rec := newReconciler(
		&fakeClient{policy: policy},
		compiler.NewCompiler(),
		polexLister, true,
	)

	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: policy.Name}})
	require.NoError(t, err)

	eng := NewEngine(
		rec,
		func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
		matching.NewMatcher(),
		&fakeTypeConverter{},
		&libs.FakeContextProvider{},
	)

	dryRun := false

	tests := []struct {
		name          string
		gvk           metav1.GroupVersionKind
		gvr           metav1.GroupVersionResource
		rawObject     string
		containerPath []string
		expectSkip    bool
	}{
		{
			name:          "matching exception skips Deployment mutation",
			gvk:           metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			gvr:           metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			rawObject:     `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"dep-nginx","namespace":"default"},"spec":{"template":{"spec":{"containers":[{"name":"nginx","image":"nginx:1.21","securityContext":{"allowPrivilegeEscalation":true}}]}}}}`,
			containerPath: []string{"spec", "template", "spec", "containers"},
			expectSkip:    true,
		},
		{
			name:          "non-matching Deployment receives mutation",
			gvk:           metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			gvr:           metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			rawObject:     `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"dep-redis","namespace":"default"},"spec":{"template":{"spec":{"containers":[{"name":"redis","image":"redis:6","securityContext":{"allowPrivilegeEscalation":true}}]}}}}`,
			containerPath: []string{"spec", "template", "spec", "containers"},
			expectSkip:    false,
		},
		{
			name:          "matching exception skips CronJob mutation",
			gvk:           metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
			gvr:           metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
			rawObject:     `{"apiVersion":"batch/v1","kind":"CronJob","metadata":{"name":"cron-nginx","namespace":"default"},"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"name":"nginx","image":"nginx:1.21","securityContext":{"allowPrivilegeEscalation":true}}]}}}}}}`,
			containerPath: []string{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
			expectSkip:    true,
		},
		{
			name:          "non-matching CronJob receives mutation",
			gvk:           metav1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"},
			gvr:           metav1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
			rawObject:     `{"apiVersion":"batch/v1","kind":"CronJob","metadata":{"name":"cron-redis","namespace":"default"},"spec":{"jobTemplate":{"spec":{"template":{"spec":{"containers":[{"name":"redis","image":"redis:6","securityContext":{"allowPrivilegeEscalation":true}}]}}}}}}`,
			containerPath: []string{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
			expectSkip:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := celengine.EngineRequest{
				Request: admissionv1.AdmissionRequest{
					Kind:      tc.gvk,
					Resource:  tc.gvr,
					Namespace: "default",
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: []byte(tc.rawObject),
					},
					DryRun: &dryRun,
				},
			}
			resp, err := eng.Handle(context.Background(), req, nil)
			require.NoError(t, err)

			if tc.expectSkip {
				assert.Nil(t, resp.PatchedResource, "matching exception must skip mutation")
				foundSkip := false
				for _, pol := range resp.Policies {
					for _, rule := range pol.Rules {
						if rule.Status() == engineapi.RuleStatusSkip {
							foundSkip = true
							assert.Contains(t, rule.Message(), "exempt-nginx")
						}
					}
				}
				assert.True(t, foundSkip, "expected rule skipped by exception")
			} else {
				require.NotNil(t, resp.PatchedResource, "non-matching resource must receive expected mutation")
				containers, found, _ := unstructured.NestedSlice(resp.PatchedResource.Object, tc.containerPath...)
				require.True(t, found)
				require.Len(t, containers, 1)
				containerMap := containers[0].(map[string]any)
				secCtx, found, _ := unstructured.NestedMap(containerMap, "securityContext")
				require.True(t, found)
				assert.Equal(t, false, secCtx["allowPrivilegeEscalation"])
			}
		})
	}
}
