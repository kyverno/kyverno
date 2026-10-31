package engine

import (
	"context"
	"testing"

	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestStoredCredentialScopeViolationRemainsEnforced(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	policy := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-credentials", Namespace: "tenant-a"},
		Spec:       autogennablePolicy("tenant-credentials").Spec,
	}
	policy.Spec.Credentials = &policiesv1beta1.Credentials{Secrets: []string{"tenant-b/registry"}}
	policy.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
	policy.Spec.ValidationConfigurations = policiesv1alpha1.ValidationConfiguration{VerifyDigest: ptr.To(false), Required: ptr.To(false), MutateDigest: ptr.To(false)}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(policy).Build()
	reconciler := newReconciler(eval.NewCompiler(nil), client, nil, false)
	key := types.NamespacedName{Namespace: policy.Namespace, Name: policy.Name}
	request := reconcile.Request{NamespacedName: key}
	_, err := reconciler.Reconcile(ctx, request)
	require.NoError(t, err)
	entries, err := reconciler.Fetch(ctx)
	require.NoError(t, err)
	require.Greater(t, len(entries), 1, "invalid policies retain autogen enforcement too")
	for _, entry := range entries {
		require.ErrorContains(t, entry.ScopeError, "instead of policy namespace")
		require.Nil(t, entry.CompiledPolicy)
	}
	engine := reusableEngine(reconciler)
	admission := reuseRequest()
	admission.Request.Namespace = policy.Namespace
	response, err := engine.HandleValidating(ctx, admission, nil)
	require.NoError(t, err)
	require.Len(t, response.Policies, 1)
	require.Equal(t, engineapi.RuleStatusError, response.Policies[0].Result.Status())
	require.Contains(t, response.Policies[0].Result.Message(), "instead of policy namespace")

	// Correcting the stored policy replaces every error entry with a valid program.
	require.NoError(t, client.Get(ctx, key, policy))
	policy.Spec.Credentials = nil
	require.NoError(t, client.Update(ctx, policy))
	_, err = reconciler.Reconcile(ctx, request)
	require.NoError(t, err)
	entries, err = reconciler.Fetch(ctx)
	require.NoError(t, err)
	for _, entry := range entries {
		require.NoError(t, entry.ScopeError)
		require.NotNil(t, entry.CompiledPolicy)
	}
	response, err = engine.HandleValidating(ctx, admission, nil)
	require.NoError(t, err)
	require.Len(t, response.Policies, 1)
	require.Equal(t, engineapi.RuleStatusPass, response.Policies[0].Result.Status())
	require.NoError(t, client.Delete(ctx, policy))
	_, err = reconciler.Reconcile(ctx, request)
	require.NoError(t, err)
	entries, err = reconciler.Fetch(ctx)
	require.NoError(t, err)
	require.Empty(t, entries)
}
