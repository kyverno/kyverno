package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func kubernetesMutationPolicy(name string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
					},
				}},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: `[JSONPatch{op:"add",path:"/metadata/labels",value:{"a":"b"}}]`},
			}},
		},
	}
}

// The live reconciler never loads JSON mode policies into the admission
// provider, and drops them if the mode is switched on an existing policy.
func TestReconcile_JSONModeExcluded(t *testing.T) {
	ctx := context.Background()
	jsonPolicy := jsonMutationPolicy("json-policy", `[JSONPatch{op:"add",path:"/a",value:1}]`)
	k8sPolicy := kubernetesMutationPolicy("k8s-policy")

	client := &fakeClient{policy: k8sPolicy}
	rec := newReconciler(client, compiler.NewCompiler(), nil, false)
	_, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: k8sPolicy.Name}})
	require.NoError(t, err)
	require.Len(t, rec.Fetch(ctx, false), 1)

	client.policy = jsonPolicy
	_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: jsonPolicy.Name}})
	require.NoError(t, err)
	require.Len(t, rec.Fetch(ctx, false), 1, "JSON policy must not be added")
	require.Equal(t, "k8s-policy", rec.Fetch(ctx, false)[0].Policy.GetName())

	// Switching an already loaded policy to JSON mode evicts it.
	switched := k8sPolicy.DeepCopy()
	switched.Spec.EvaluationConfiguration = jsonPolicy.Spec.EvaluationConfiguration
	switched.Spec.MatchConstraints = nil
	client.policy = switched
	_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: k8sPolicy.Name}})
	require.NoError(t, err)
	require.Empty(t, rec.Fetch(ctx, false))

	// A namespaced JSON policy is excluded the same way.
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json-ns", Namespace: "ns"},
		Spec:       jsonPolicy.Spec,
	}
	rec = newReconciler(&fakeClient{nmpol: nmpol}, compiler.NewCompiler(), nil, false)
	_, err = rec.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "json-ns"}})
	require.NoError(t, err)
	require.Empty(t, rec.Fetch(ctx, false))
}

// NewProvider is the Kubernetes admission provider: it must fail on a JSON
// policy rather than silently treating it as an admission policy.
func TestNewProvider_RejectsJSONMode(t *testing.T) {
	jsonPolicy := jsonMutationPolicy("json-policy", `[JSONPatch{op:"add",path:"/a",value:1}]`)
	_, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{jsonPolicy}, nil, libs.NewFakeContextProvider())
	require.ErrorContains(t, err, "JSON mode")

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{kubernetesMutationPolicy("k8s-policy")}, nil, libs.NewFakeContextProvider())
	require.NoError(t, err)
	require.Len(t, provider.Fetch(context.Background(), false), 1)

	// The JSON engine accepts the JSON policy and rejects the Kubernetes one.
	_, err = NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{jsonPolicy}, nil)
	require.NoError(t, err)
	_, err = NewJSONEngine([]policiesv1beta1.MutatingPolicyLike{kubernetesMutationPolicy("k8s-policy")}, nil)
	require.Error(t, err)
}

// Switching a policy from JSON mode back to Kubernetes mode makes the
// reconciler load it into the admission provider again.
func TestReconcile_JSONToKubernetesModeSwitch(t *testing.T) {
	ctx := context.Background()
	jsonPolicy := jsonMutationPolicy("switch", `[JSONPatch{op:"add",path:"/a",value:1}]`)
	client := &fakeClient{policy: jsonPolicy}
	rec := newReconciler(client, compiler.NewCompiler(), nil, false)
	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: "switch"}}
	_, err := rec.Reconcile(ctx, req)
	require.NoError(t, err)
	require.Empty(t, rec.Fetch(ctx, false))

	client.policy = kubernetesMutationPolicy("switch")
	_, err = rec.Reconcile(ctx, req)
	require.NoError(t, err)
	fetched := rec.Fetch(ctx, false)
	require.Len(t, fetched, 1)
	require.Equal(t, "switch", fetched[0].Policy.GetName())
}
