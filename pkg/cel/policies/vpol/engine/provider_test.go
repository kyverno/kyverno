package engine

import (
	"context"
	"slices"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
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
		&policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-a"}},
		&policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-b"}},
		&policiesv1beta1.NamespacedValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-a"}},
	).Build()
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "require-team", Kind: policieskyvernoio.NamespacedValidatingPolicyKind},
				{Name: "cluster-policy", Kind: policieskyvernoio.ValidatingPolicyKind},
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
			refs = append(refs, policiesv1beta1.PolicyRef{Name: name, Kind: policieskyvernoio.ValidatingPolicyKind})
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

// TestNewProvider_AutogenExceptionRewrite is a regression test for #16952:
// a PolicyException's matchConditions must be rewritten the same way the
// policy's own expressions are for each autogen'd controller variant.
func TestNewProvider_AutogenExceptionRewrite(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disallow-privilege-escalation"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"daemonsets"},
				},
			},
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.containers.all(c, !has(c.securityContext) || !has(c.securityContext.allowPrivilegeEscalation) || c.securityContext.allowPrivilegeEscalation == false)"},
			},
		},
	}

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exclude-privileged"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "disallow-privilege-escalation", Kind: "ValidatingPolicy"},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "privileged",
					Expression: "object.spec.containers.exists(c, has(c.securityContext) && has(c.securityContext.allowPrivilegeEscalation) && c.securityContext.allowPrivilegeEscalation == true)",
				},
			},
		},
	}

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, []*policiesv1beta1.PolicyException{exception}, celengine.ExceptionScopeAll)
	require.NoError(t, err)

	policies, err := provider(context.Background())
	require.NoError(t, err)

	var daemonsetPolicy *Policy
	for i := range policies {
		spec := policies[i].Policy.GetValidatingPolicySpec()
		if spec.MatchConstraints != nil && len(spec.MatchConstraints.ResourceRules) > 0 &&
			slices.Contains(spec.MatchConstraints.ResourceRules[0].Resources, "daemonsets") {
			daemonsetPolicy = &policies[i]
			break
		}
	}
	require.NotNil(t, daemonsetPolicy, "expected an autogenerated daemonsets policy variant")

	tests := []struct {
		name            string
		object          map[string]any
		expectException bool
	}{
		{
			name: "daemonset matching exception",
			object: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name": "agent",
									"securityContext": map[string]any{
										"allowPrivilegeEscalation": true,
									},
								},
							},
						},
					},
				},
			},
			expectException: true,
		},
		{
			name: "daemonset not matching exception",
			object: map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": map[string]any{
							"containers": []any{
								map[string]any{
									"name": "agent",
									"securityContext": map[string]any{
										"allowPrivilegeEscalation": false,
									},
								},
							},
						},
					},
				},
			},
			expectException: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := daemonsetPolicy.CompiledPolicy.Evaluate(context.Background(), tt.object, nil, nil, nil, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, result)
			if tt.expectException {
				assert.NotEmpty(t, result.Exceptions, "the privileged daemonset should be excepted")
				if len(result.Exceptions) > 0 {
					assert.Equal(t, "exclude-privileged", result.Exceptions[0].Name, "should match the exact exception identity")
				}
			} else {
				assert.Empty(t, result.Exceptions, "the unprivileged daemonset should not be excepted")
			}
		})
	}
}

// TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions pins the namespace argument
// NewProvider passes to scope.Select: under scope "*" a namespaced policy must only accept
// exceptions filed in its own namespace, never a foreign one. It fails if NewProvider ever stops
// passing policy.GetNamespace() through to the exception match.
func TestNewProvider_NamespacedPolicyOnlyAcceptsOwnNamespaceExceptions(t *testing.T) {
	policy := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.name == 'forbidden'", Message: "denied"},
			},
		},
	}
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedValidatingPolicy"}},
			Namespaces: []string{"tenant-a"},
		},
	}
	foreignException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "foreign", Namespace: "tenant-b"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedValidatingPolicy"}},
			Namespaces: []string{"tenant-a"},
		},
	}

	provider, err := NewProvider(
		compiler.NewCompiler(),
		[]policiesv1beta1.ValidatingPolicyLike{policy},
		[]*policiesv1beta1.PolicyException{ownException, foreignException},
		celengine.ExceptionScopeAll,
	)
	require.NoError(t, err)

	policies, err := provider(context.Background())
	require.NoError(t, err)
	require.Len(t, policies, 1)

	result, err := policies[0].CompiledPolicy.Evaluate(context.Background(), map[string]any{"name": "forbidden"}, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, result.Exceptions, 1, "tenant-a's policy must only see its own namespace's exception")
	assert.Equal(t, "tenant-a", result.Exceptions[0].GetNamespace())
	assert.Equal(t, "own", result.Exceptions[0].GetName())
}

// TestNewProvider_ScopeGatesExceptionSelection pins the scope argument NewProvider passes to
// scope.Select: it must not be ignored or hardcoded to ExceptionScopeAll. ExceptionScopeNone must
// select nothing, and a concrete scope must select only the exception filed in that (central)
// namespace, even though the policy's own namespace would otherwise qualify.
func TestNewProvider_ScopeGatesExceptionSelection(t *testing.T) {
	policy := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "guard", Namespace: "tenant-a"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.name == 'forbidden'", Message: "denied"},
			},
		},
	}
	ownException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "own", Namespace: "tenant-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedValidatingPolicy"}},
			Namespaces: []string{"tenant-a"},
		},
	}
	centralException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "central", Namespace: "central-ns"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "guard", Kind: "NamespacedValidatingPolicy"}},
			Namespaces: []string{"tenant-a"},
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
			provider, err := NewProvider(
				compiler.NewCompiler(),
				[]policiesv1beta1.ValidatingPolicyLike{policy},
				[]*policiesv1beta1.PolicyException{ownException, centralException},
				tt.scope,
			)
			require.NoError(t, err)

			policies, err := provider(context.Background())
			require.NoError(t, err)
			require.Len(t, policies, 1)

			result, err := policies[0].CompiledPolicy.Evaluate(context.Background(), map[string]any{"name": "forbidden"}, nil, nil, nil, nil, nil)
			require.NoError(t, err)

			if len(tt.expectedNames) == 0 {
				assert.Empty(t, result.Exceptions)
				return
			}
			require.Len(t, result.Exceptions, len(tt.expectedNames))
			for i, name := range tt.expectedNames {
				assert.Equal(t, name, result.Exceptions[i].GetName())
			}
		})
	}
}
