package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type fakeCompiledPolicy struct{}

func (f *fakeCompiledPolicy) MatchesConditions(_ context.Context, _ admission.Attributes, _ *admissionv1.AdmissionRequest, _ *corev1.Namespace) bool {
	return true
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name          string
		pols          []policiesv1beta1.MutatingPolicyLike
		exceptions    []*policiesv1beta1.PolicyException
		expectErr     bool
		expectedCount int
	}{
		{
			name: "valid policy without exception",
			pols: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy1"},
				},
			},
			expectErr:     false,
			expectedCount: 1, // includes autogen clone
		},
		{
			name: "policy with matching exception",
			pols: []policiesv1beta1.MutatingPolicyLike{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy-exc"},
				},
			},
			exceptions: []*policiesv1beta1.PolicyException{
				{
					Spec: policiesv1beta1.PolicyExceptionSpec{
						PolicyRefs: []policiesv1beta1.PolicyRef{
							{
								Name: "policy-exc",
								Kind: "MutatingPolicy",
							},
						},
					},
				},
			},
			expectErr:     false,
			expectedCount: 1, // includes autogen clone
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewProvider(compiler.NewCompiler(), tt.pols, tt.exceptions, libs.NewFakeContextProvider(), celengine.ExceptionScopeAll)
			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, prov)

				pols := prov.Fetch(context.Background(), false)
				assert.GreaterOrEqual(t, len(pols), tt.expectedCount)
			}
		})
	}
}

// TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument
// NewProvider passes to scope.Select: under scope "*" a namespaced policy must only accept
// exceptions filed in its own namespace, never a foreign one. It fails if NewProvider ever stops
// passing policy.GetNamespace() through to the exception match.
func TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
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

	prov, err := NewProvider(
		compiler.NewCompiler(),
		[]policiesv1beta1.MutatingPolicyLike{policy},
		[]*policiesv1beta1.PolicyException{ownException, foreignException},
		libs.NewFakeContextProvider(),
		celengine.ExceptionScopeAll,
	)
	require.NoError(t, err)

	eng := NewEngine(
		prov,
		func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
		matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{},
	)
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
	resp, err := eng.Handle(context.Background(), req, predicate)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)
	rule := resp.Policies[0].Rules[0]
	require.Len(t, rule.Exceptions(), 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", rule.Exceptions()[0].GetNamespace())
	assert.Equal(t, "own", rule.Exceptions()[0].GetName())
}

// TestNewProvider_ScopeGatesExceptionSelection pins the scope argument NewProvider passes to
// scope.Select: it must not be ignored or hardcoded to ExceptionScopeAll. ExceptionScopeNone must
// select nothing, and a concrete scope must select only the exception filed in that (central)
// namespace, even though the policy's own namespace would otherwise qualify.
func TestNewProvider_ScopeGatesExceptionSelection(t *testing.T) {
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
	centralException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "central", Namespace: "central-ns"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedMutatingPolicy"}},
		},
	}

	tests := []struct {
		name          string
		scope         celengine.ExceptionScope
		expectedNames []string
	}{
		{name: "scope none selects nothing", scope: celengine.ExceptionScopeNone},
		{name: "concrete scope selects only the central namespace's exception", scope: celengine.ExceptionScope("central-ns"), expectedNames: []string{"central"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewProvider(
				compiler.NewCompiler(),
				[]policiesv1beta1.MutatingPolicyLike{policy},
				[]*policiesv1beta1.PolicyException{ownException, centralException},
				libs.NewFakeContextProvider(),
				tt.scope,
			)
			require.NoError(t, err)

			eng := NewEngine(
				prov,
				func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
				matcher, &fakeTypeConverter{}, &libs.FakeContextProvider{},
			)
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
			resp, err := eng.Handle(context.Background(), req, predicate)
			require.NoError(t, err)
			require.Len(t, resp.Policies, 1)
			require.Len(t, resp.Policies[0].Rules, 1)
			rule := resp.Policies[0].Rules[0]

			if len(tt.expectedNames) == 0 {
				assert.Empty(t, rule.Exceptions())
				return
			}
			require.Len(t, rule.Exceptions(), len(tt.expectedNames))
			for i, name := range tt.expectedNames {
				assert.Equal(t, name, rule.Exceptions()[i].GetName())
			}
		})
	}
}

func TestStaticProviderFetch(t *testing.T) {
	trueBool := true
	falseBool := false

	policy1 := policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "enabled-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
		},
	}
	policy2 := policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disabled-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &falseBool,
				},
			},
		},
	}

	provider := &staticProvider{
		policies: []Policy{
			{Policy: &policy1},
			{Policy: &policy2},
		},
	}

	// The static provider must mirror the cluster reconciler's Fetch semantics:
	// mutateExisting=true returns only the mutate-existing policies, while
	// mutateExisting=false returns all policies (a mutate-existing policy still
	// runs on admission by default). Filtering false down to non-mutate-existing
	// policies drops them from the CLI admission path and from the background
	// reports scanner, which both build this provider and call Fetch(ctx, false).
	t.Run("fetch mutateExisting == true returns only mutate-existing policies", func(t *testing.T) {
		res := provider.Fetch(context.TODO(), true)
		assert.Len(t, res, 1)
		assert.Equal(t, "enabled-policy", res[0].Policy.GetName())
	})

	t.Run("fetch mutateExisting == false returns all policies", func(t *testing.T) {
		res := provider.Fetch(context.TODO(), false)
		var names []string
		for _, p := range res {
			names = append(names, p.Policy.GetName())
		}
		assert.Len(t, res, 2)
		assert.Contains(t, names, "enabled-policy")
		assert.Contains(t, names, "disabled-policy")
	})
}

func TestStaticProviderMatchesMutateExisting(t *testing.T) {
	trueBool := true

	matchPolicy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "match"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
				MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
					Enabled: &trueBool,
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{}, // match everything
		},
	}
	provider := &staticProvider{
		policies: []Policy{
			{
				Policy:         matchPolicy,
				CompiledPolicy: &compiler.Policy{},
			},
		},
	}

	t.Run("match all", func(t *testing.T) {
		names := provider.MatchesMutateExisting(context.Background(), &mockAttributes{}, nil, &corev1.Namespace{}, nil)
		assert.Equal(t, []string{"match"}, names)
	})
}

// TestStaticProviderMatchesMutateExisting_BuildsRequestMapAtMostOnce is the
// CI-assertable regression guard for the mutate-existing matching hoist:
// with N candidate policies that all have matchConditions (so
// MatchesConditions, and therefore prepareData, is actually invoked for
// each one), the caller-supplied requestMapFn - a real sync.OnceValues
// memoized thunk, exactly what engineImpl.MatchedMutateExistingPolicies
// builds - must be invoked at most once across the whole matching pass, not
// once per policy.
func TestStaticProviderMatchesMutateExisting_BuildsRequestMapAtMostOnce(t *testing.T) {
	trueBool := true
	const n = 5

	comp := compiler.NewCompiler()
	policies := make([]Policy, 0, n)
	for i := 0; i < n; i++ {
		mpol := &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("match-%d", i)},
			Spec: policiesv1beta1.MutatingPolicySpec{
				EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
					MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{
						Enabled: &trueBool,
					},
				},
				MatchConstraints: &admissionregistrationv1.MatchResources{}, // match everything
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{Name: "always-true", Expression: "true"},
				},
			},
		}
		compiled, errs := comp.Compile(mpol, nil)
		require.Empty(t, errs)
		policies = append(policies, Policy{Policy: mpol, CompiledPolicy: compiled})
	}
	provider := &staticProvider{policies: policies}

	request := &admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Name:      "nginx",
		Namespace: "default",
		Operation: admissionv1.Create,
	}

	calls := 0
	requestMapFn := sync.OnceValues(func() (map[string]any, error) {
		calls++
		return celcompiler.BuildNormalizedRequestMap(request)
	})

	names := provider.MatchesMutateExisting(context.Background(), &mockAttributes{}, request, &corev1.Namespace{}, requestMapFn)
	assert.Len(t, names, n, "all %d policies must have matched", n)
	assert.Equal(t, 1, calls,
		"requestMapFn must be invoked at most once across all %d candidate policies, not once per policy", n)
}

func TestPolicyExceptionHandler_RequeuesNamespacedPoliciesWithNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "add-label", Namespace: "team-a"}},
		&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "add-label", Namespace: "team-b"}},
		&policiesv1beta1.NamespacedMutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-a"}},
	).Build()
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "add-label", Kind: "NamespacedMutatingPolicy"},
				{Name: "cluster-policy", Kind: "MutatingPolicy"},
			},
		},
	}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	newPolicyExceptionHandler(c).Create(context.Background(), event.TypedCreateEvent[client.Object]{Object: polex}, q)

	var got []reconcile.Request
	for q.Len() > 0 {
		item, _ := q.Get()
		got = append(got, item)
		q.Done(item)
	}
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKey{Namespace: "team-a", Name: "add-label"}},
		{NamespacedName: client.ObjectKey{Namespace: "team-b", Name: "add-label"}},
		{NamespacedName: client.ObjectKey{Name: "cluster-policy"}},
	}, got)
}

func TestPolicyExceptionHandler_RequeuesOldAndNewRefs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	polex := func(names ...string) *policiesv1beta1.PolicyException {
		var refs []policiesv1beta1.PolicyRef
		for _, name := range names {
			refs = append(refs, policiesv1beta1.PolicyRef{Name: name, Kind: "MutatingPolicy"})
		}
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
			Spec:       policiesv1beta1.PolicyExceptionSpec{PolicyRefs: refs},
		}
	}
	drain := func(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
		var got []reconcile.Request
		for q.Len() > 0 {
			item, _ := q.Get()
			got = append(got, item)
			q.Done(item)
		}
		return got
	}

	t.Run("update requeues refs removed from the exception", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		defer q.ShutDown()
		newPolicyExceptionHandler(c).Update(context.Background(), event.TypedUpdateEvent[client.Object]{
			ObjectOld: polex("policy-a", "policy-b"),
			ObjectNew: polex("policy-a"),
		}, q)
		assert.ElementsMatch(t, []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "policy-a"}},
			{NamespacedName: client.ObjectKey{Name: "policy-b"}},
		}, drain(q))
	})

	t.Run("delete requeues the refs", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		defer q.ShutDown()
		newPolicyExceptionHandler(c).Delete(context.Background(), event.TypedDeleteEvent[client.Object]{
			Object: polex("policy-a"),
		}, q)
		assert.ElementsMatch(t, []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "policy-a"}},
		}, drain(q))
	})
}
