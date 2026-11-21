package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func warmUpTestScheme(t *testing.T) *kruntime.Scheme {
	t.Helper()
	scheme := kruntime.NewScheme()
	require.NoError(t, policiesv1beta1.Install(scheme))
	return scheme
}

// mutationPolicy returns a MutatingPolicy with a trivial mutation expression, so it compiles
// successfully.
func mutationPolicy(name string) *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
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
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: `Object{metadata: Object.metadata{labels: {"warmed-up": "true"}}}`,
					},
				},
			},
		},
	}
}

// TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent is the regression test for the CEL engine
// startup enforcement gap: pre-existing MutatingPolicy/NamespacedMutatingPolicy
// objects must be compiled and visible to Fetch() synchronously, without waiting on any
// workqueue-driven Reconcile call.
func TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent(t *testing.T) {
	scheme := warmUpTestScheme(t)
	cpol := mutationPolicy("cluster-policy")
	npol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-policy", Namespace: "default"},
		Spec:       cpol.Spec,
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cpol, npol).
		Build()

	rec := newReconciler(cl, compiler.NewCompiler(), nil, false)

	before := rec.Fetch(context.Background(), false)
	require.Empty(t, before, "no policy should be visible before WarmUp or Reconcile ran")

	require.NoError(t, rec.WarmUp(context.Background()))

	after := rec.Fetch(context.Background(), false)
	names := map[string]bool{}
	for _, p := range after {
		names[p.Policy.GetName()] = true
	}
	assert.True(t, names["cluster-policy"])
	assert.True(t, names["ns-policy"])
}

// TestWarmUp_NoPolicies ensures WarmUp is a no-op (and doesn't error) when no policies exist yet.
func TestWarmUp_NoPolicies(t *testing.T) {
	scheme := warmUpTestScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	rec := newReconciler(cl, compiler.NewCompiler(), nil, false)
	require.NoError(t, rec.WarmUp(context.Background()))

	assert.Empty(t, rec.Fetch(context.Background(), false))
}

// TestWarmUp_InvalidPolicyDoesNotBlockOthers ensures a policy that no longer compiles neither fails
// WarmUp (which would crash-loop the admission controller) nor prevents other policies from being
// warmed up. Unlike vpol, the mpol Reconcile returns compile errors, so WarmUp must tolerate them.
func TestWarmUp_InvalidPolicyDoesNotBlockOthers(t *testing.T) {
	scheme := warmUpTestScheme(t)
	broken := mutationPolicy("a-broken-policy")
	broken.Spec.Mutations[0].ApplyConfiguration.Expression = "this is not ( valid cel"

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(broken, mutationPolicy("cluster-policy")).
		Build()

	rec := newReconciler(cl, compiler.NewCompiler(), nil, false)
	require.NoError(t, rec.WarmUp(context.Background()))

	names := map[string]bool{}
	for _, p := range rec.Fetch(context.Background(), false) {
		names[p.Policy.GetName()] = true
	}
	assert.Equal(t, map[string]bool{"cluster-policy": true}, names)
}
