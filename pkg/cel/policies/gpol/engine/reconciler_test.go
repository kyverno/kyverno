package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *kruntime.Scheme {
	t.Helper()
	scheme := kruntime.NewScheme()
	require.NoError(t, policiesv1beta1.Install(scheme))
	return scheme
}

func newTestReconciler(t *testing.T, objs ...client.Object) *reconciler {
	t.Helper()
	fakeClient := fake.NewClientBuilder().
		WithScheme(newTestScheme(t)).
		WithObjects(objs...).
		Build()
	comp := compiler.NewCompiler()
	return newReconciler(comp, fakeClient, nil, false)
}

func minimalGpol(name string) *policiesv1beta1.GeneratingPolicy {
	return &policiesv1beta1.GeneratingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GeneratingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func minimalNgpol(namespace, name string) *policiesv1beta1.NamespacedGeneratingPolicy {
	return &policiesv1beta1.NamespacedGeneratingPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NamespacedGeneratingPolicy",
			APIVersion: "policies.kyverno.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestReconciler_Get_CacheMiss(t *testing.T) {
	r := newTestReconciler(t)
	_, err := r.Get(context.Background(), "does-not-exist")
	assert.ErrorContains(t, err, "not found in cache")
}

func TestReconciler_Reconcile_StoresCompiledPolicy(t *testing.T) {
	gpol := minimalGpol("my-policy")
	r := newTestReconciler(t, gpol)

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-policy"},
	})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result)

	policy, err := r.Get(context.Background(), "my-policy")
	require.NoError(t, err)
	assert.Equal(t, "my-policy", policy.Policy.GetName())
	assert.NotNil(t, policy.CompiledPolicy)
}

func TestReconciler_Reconcile_NotFound_RemovesFromCache(t *testing.T) {
	gpol := minimalGpol("my-policy")
	r := newTestReconciler(t, gpol)

	// Populate cache.
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-policy"},
	})
	require.NoError(t, err)
	_, err = r.Get(context.Background(), "my-policy")
	require.NoError(t, err)

	// Delete from fake client so next Reconcile gets NotFound.
	require.NoError(t, r.client.Delete(context.Background(), gpol))

	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "my-policy"},
	})
	require.NoError(t, err)

	_, err = r.Get(context.Background(), "my-policy")
	assert.ErrorContains(t, err, "not found in cache")
}

func TestReconciler_Reconcile_UpdatesExistingEntry(t *testing.T) {
	gpol := minimalGpol("my-policy")
	r := newTestReconciler(t, gpol)

	for range 2 {
		_, err := r.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{Name: "my-policy"},
		})
		require.NoError(t, err)
	}

	policy, err := r.Get(context.Background(), "my-policy")
	require.NoError(t, err)
	assert.Equal(t, "my-policy", policy.Policy.GetName())
}

func TestReconciler_Reconcile_NamespacedPolicy(t *testing.T) {
	ngpol := minimalNgpol("team-a", "ns-policy")
	r := newTestReconciler(t, ngpol)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "team-a", Name: "ns-policy"},
	})
	require.NoError(t, err)

	// key must be "namespace/name" — matches GetPolicyKey format from the webhook handler.
	policy, err := r.Get(context.Background(), "team-a/ns-policy")
	require.NoError(t, err)
	assert.Equal(t, "ns-policy", policy.Policy.GetName())
	assert.Equal(t, "team-a", policy.Policy.GetNamespace())
}

func TestReconciler_Get_ConcurrentReads(t *testing.T) {
	gpol := minimalGpol("concurrent-policy")
	r := newTestReconciler(t, gpol)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "concurrent-policy"},
	})
	require.NoError(t, err)

	done := make(chan struct{}, 20)
	for range 20 {
		go func() {
			defer func() { done <- struct{}{} }()
			p, err := r.Get(context.Background(), "concurrent-policy")
			assert.NoError(t, err)
			assert.NotNil(t, p.CompiledPolicy)
		}()
	}
	for range 20 {
		<-done
	}
}

func TestReconciler_Get_ClusterAndNamespacedCoexist(t *testing.T) {
	gpol := minimalGpol("cluster-policy")
	ngpol := minimalNgpol("ns-a", "ns-policy")
	r := newTestReconciler(t, client.Object(gpol), client.Object(ngpol))

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "cluster-policy"},
	})
	require.NoError(t, err)

	_, err = r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns-a", Name: "ns-policy"},
	})
	require.NoError(t, err)

	clusterPol, err := r.Get(context.Background(), "cluster-policy")
	require.NoError(t, err)
	assert.Equal(t, "cluster-policy", clusterPol.Policy.GetName())

	nsPol, err := r.Get(context.Background(), "ns-a/ns-policy")
	require.NoError(t, err)
	assert.Equal(t, "ns-policy", nsPol.Policy.GetName())
}
