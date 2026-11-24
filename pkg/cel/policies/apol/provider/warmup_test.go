package provider

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	apolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/apol/compiler"
	apolengine "github.com/kyverno/kyverno/pkg/cel/policies/apol/engine"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestReconciler(t *testing.T, objs ...*policiesv1alpha1.AuthorizingPolicy) *reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1alpha1.Install(scheme))
	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&policiesv1alpha1.AuthorizingPolicy{})
	for _, obj := range objs {
		builder = builder.WithObjects(obj)
	}
	return &reconciler{client: builder.Build(), compiler: apolcompiler.NewCompiler(), policies: map[string]apolengine.CompiledPolicy{}}
}

func TestWarmUpDoesNotResurrectDeletedPolicies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kept := &policiesv1alpha1.AuthorizingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kept"}}
	deleted := &policiesv1alpha1.AuthorizingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "deleted"}}
	r := newTestReconciler(t, kept, deleted)
	// A reconcile adds the policy before warm-up, then the policy is deleted
	// from the cache before warm-up lists it.
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "deleted"}})
	require.NoError(t, err)
	require.NoError(t, r.client.Delete(ctx, deleted.DeepCopy()))
	require.NoError(t, r.WarmUp(ctx))
	snapshot, err := r.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, snapshot.Len())
	require.Contains(t, r.policies, "/kept")
	require.NotContains(t, r.policies, "/deleted")

	// After warm-up, a delete reconcile removes the policy and publishes.
	require.NoError(t, r.client.Delete(ctx, kept.DeepCopy()))
	_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "kept"}})
	require.NoError(t, err)
	snapshot, err = r.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, snapshot.Len())
}
