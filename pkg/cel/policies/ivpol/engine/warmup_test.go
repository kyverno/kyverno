package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	iveval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

func warmUpFetchedNames(t *testing.T, rec *reconciler) map[string]bool {
	t.Helper()
	fetched, err := rec.Fetch(context.Background())
	require.NoError(t, err)
	names := map[string]bool{}
	for _, p := range fetched {
		names[p.Policy.GetName()] = true
	}
	return names
}

// TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent is the regression test for the CEL engine
// startup enforcement gap: pre-existing ImageValidatingPolicy/NamespacedImageValidatingPolicy
// objects must be compiled and visible to Fetch() synchronously, without waiting on any
// workqueue-driven Reconcile call.
func TestWarmUp_PopulatesPoliciesBeforeAnyReconcileEvent(t *testing.T) {
	cpol := autogennablePolicy("cluster-policy")
	npol := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ns-policy", Namespace: "default"},
		Spec:       cpol.Spec,
	}
	cl := fake.NewClientBuilder().
		WithScheme(warmUpTestScheme(t)).
		WithObjects(cpol, npol).
		Build()
	rec := newReconciler(iveval.NewCompiler(nil), cl, nil, false)

	require.Empty(t, warmUpFetchedNames(t, rec), "no policy should be visible before WarmUp or Reconcile ran")
	require.NoError(t, rec.WarmUp(context.Background()))
	assert.Equal(t, map[string]bool{"cluster-policy": true, "ns-policy": true}, warmUpFetchedNames(t, rec))
}

// TestWarmUp_NoPolicies ensures WarmUp is a no-op (and doesn't error) when no policies exist yet.
func TestWarmUp_NoPolicies(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(warmUpTestScheme(t)).Build()
	rec := newReconciler(iveval.NewCompiler(nil), cl, nil, false)

	require.NoError(t, rec.WarmUp(context.Background()))
	assert.Empty(t, warmUpFetchedNames(t, rec))
}

// TestWarmUp_InvalidPolicyDoesNotBlockOthers ensures a policy that no longer compiles neither fails
// WarmUp (which would crash-loop the admission controller) nor prevents other policies from being
// warmed up. Unlike vpol, the ivpol Reconcile returns compile errors, so WarmUp must tolerate them.
func TestWarmUp_InvalidPolicyDoesNotBlockOthers(t *testing.T) {
	broken := autogennablePolicy("a-broken-policy")
	broken.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "this is not ( valid cel"}}
	cl := fake.NewClientBuilder().
		WithScheme(warmUpTestScheme(t)).
		WithObjects(broken, autogennablePolicy("cluster-policy")).
		Build()
	rec := newReconciler(iveval.NewCompiler(nil), cl, nil, false)

	require.NoError(t, rec.WarmUp(context.Background()))
	assert.Equal(t, map[string]bool{"cluster-policy": true}, warmUpFetchedNames(t, rec))
}
