package engine

import (
	"context"
	"slices"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPolicyExceptionHandler_RequeuesNamespacedPoliciesWithNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-a"}},
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-b"}},
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-a"}},
	).WithIndex(&policiesv1beta1.NamespacedImageValidatingPolicy{}, engine.NamespacedPolicyNameIndex, engine.IndexPolicyName).Build()
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "require-team", Kind: policieskyvernoio.NamespacedImageValidatingPolicyKind},
				{Name: "cluster-policy", Kind: policieskyvernoio.ImageValidatingPolicyKind},
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
		{NamespacedName: client.ObjectKey{Namespace: "team-a", Name: "require-team"}},
		{NamespacedName: client.ObjectKey{Namespace: "team-b", Name: "require-team"}},
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
			refs = append(refs, policiesv1beta1.PolicyRef{Name: name, Kind: policieskyvernoio.ImageValidatingPolicyKind})
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

// TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument
// NewProvider passes to scope.Select: under scope "*" a namespaced policy must only accept
// exceptions filed in its own namespace, never a foreign one. It fails if NewProvider ever stops
// passing policy.GetNamespace() through to the exception match.
func TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	policy := namespacedAutogennablePolicy("guard", "tenant-a")
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedImageValidatingPolicy"}},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedImageValidatingPolicy"}},
		},
	}

	provider, err := NewProvider(
		iveval.NewCompiler(nil),
		[]policiesv1beta1.ImageValidatingPolicyLike{policy},
		[]*policiesv1beta1.PolicyException{ownException, foreignException},
		engine.ExceptionScopeAll,
	)
	require.NoError(t, err)

	policies, err := provider(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, policies)
	for _, p := range policies {
		require.Len(t, p.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
		assert.Equal(t, "tenant-a", p.Exceptions[0].GetNamespace())
		assert.Equal(t, "own", p.Exceptions[0].GetName())
	}
}

// TestNewProvider_ScopeGatesExceptionSelection pins the scope argument NewProvider passes to
// scope.Select: it must not be ignored or hardcoded to ExceptionScopeAll. ExceptionScopeNone must
// select nothing, and a concrete scope must select only the exception filed in that (central)
// namespace, even though the policy's own namespace would otherwise qualify.
func TestNewProvider_ScopeGatesExceptionSelection(t *testing.T) {
	policy := namespacedAutogennablePolicy("guard", "tenant-a")
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedImageValidatingPolicy"}},
		},
	}
	centralException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "central", Namespace: "central-ns"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedImageValidatingPolicy"}},
		},
	}

	tests := []struct {
		name          string
		scope         engine.ExceptionScope
		expectedNames []string
	}{
		{name: "scope none selects nothing", scope: engine.ExceptionScopeNone},
		{name: "concrete scope selects only the central namespace's exception", scope: engine.ExceptionScope("central-ns"), expectedNames: []string{"central"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(
				iveval.NewCompiler(nil),
				[]policiesv1beta1.ImageValidatingPolicyLike{policy},
				[]*policiesv1beta1.PolicyException{ownException, centralException},
				tt.scope,
			)
			require.NoError(t, err)

			policies, err := provider(context.Background())
			require.NoError(t, err)
			require.NotEmpty(t, policies)
			for _, p := range policies {
				if len(tt.expectedNames) == 0 {
					assert.Empty(t, p.Exceptions)
					continue
				}
				require.Len(t, p.Exceptions, len(tt.expectedNames))
				for i, name := range tt.expectedNames {
					assert.Equal(t, name, p.Exceptions[i].GetName())
				}
			}
		})
	}
}

func TestNewProvider_DeterministicAutogenOrdering(t *testing.T) {
	policy := autogennablePolicy("pod-policy")
	compiler := iveval.NewCompiler(nil)

	var baseline []string
	for i := 0; i < 50; i++ {
		prov, err := NewProvider(compiler, []policiesv1beta1.ImageValidatingPolicyLike{policy}, nil, engine.ExceptionScopeAll)
		require.NoError(t, err)
		fetched, err := prov.Fetch(context.Background())
		require.NoError(t, err)

		order := variantOrder(fetched)
		if i == 0 {
			baseline = order
			require.Greater(t, len(baseline), 1, "expected autogennable policy to generate variants")
		} else {
			assert.Equal(t, baseline, order, "NewProvider autogen variants order must be deterministic at iteration %d", i)
		}
	}
}

func TestNewProvider_AutogeneratedExceptionRewriting(t *testing.T) {
	policy := autogennablePolicy("check-image")
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt-nginx"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: policy.Name,
				Kind: policieskyvernoio.ImageValidatingPolicyKind,
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "match-nginx",
					Expression: "object.spec.containers.exists(c, c.name == 'nginx')",
				},
			},
		},
	}

	capComp := &captureCompiler{Compiler: iveval.NewCompiler(nil)}
	prov, err := NewProvider(capComp, []policiesv1beta1.ImageValidatingPolicyLike{policy}, []*policiesv1beta1.PolicyException{polex}, engine.ExceptionScopeAll)
	require.NoError(t, err)

	fetched, err := prov.Fetch(context.Background())
	require.NoError(t, err)
	require.Greater(t, len(fetched), 1)

	sawDeploymentVariant := false
	sawCronJobVariant := false

	for _, p := range fetched {
		mc := p.Policy.GetSpec().MatchConstraints
		if mc == nil || len(mc.ResourceRules) == 0 {
			continue
		}
		resources := mc.ResourceRules[0].Resources
		if slices.Contains(resources, "deployments") {
			sawDeploymentVariant = true
			require.Len(t, p.Exceptions, 1)
			assert.Equal(t, "object.spec.template.spec.containers.exists(c, c.name == 'nginx')", p.Exceptions[0].Spec.MatchConditions[0].Expression)
		}
		if slices.Contains(resources, "cronjobs") {
			sawCronJobVariant = true
			require.Len(t, p.Exceptions, 1)
			assert.Equal(t, "object.spec.jobTemplate.spec.template.spec.containers.exists(c, c.name == 'nginx')", p.Exceptions[0].Spec.MatchConditions[0].Expression)
		}
	}

	assert.True(t, sawDeploymentVariant, "expected deployments variant with rewritten exception in NewProvider output")
	assert.True(t, sawCronJobVariant, "expected cronjobs variant with rewritten exception in NewProvider output")
}
