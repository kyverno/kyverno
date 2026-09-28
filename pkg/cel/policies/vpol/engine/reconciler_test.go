package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type fakeClient struct {
	client.Client
	policy *policiesv1beta1.ValidatingPolicy
	nvpol  *policiesv1beta1.NamespacedValidatingPolicy
	err    error
}

func (f *fakeClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	if f.err != nil {
		return f.err
	}
	switch o := obj.(type) {
	case *policiesv1beta1.ValidatingPolicy:
		if f.policy != nil && key.Name == f.policy.Name && key.Namespace == f.policy.Namespace {
			*o = *f.policy
			return nil
		}
	case *policiesv1beta1.NamespacedValidatingPolicy:
		if f.nvpol != nil && key.Name == f.nvpol.Name && key.Namespace == f.nvpol.Namespace {
			*o = *f.nvpol
			return nil
		}
	}
	return apierrors.NewNotFound(schema.GroupResource{}, "")
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("compilation failure removes policy from cache", func(t *testing.T) {
		vp := &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				Rules: []policiesv1beta1.ValidatingRule{
					{
						Name: "invalid-rule",
						MatchConstraints: &policiesv1beta1.MatchResources{
							Any: policiesv1beta1.ResourceFilters{
								{
									ResourceDescription: policiesv1beta1.ResourceDescription{
										Kinds: []string{"Pod"},
									},
								},
							},
						},
						CELPreconditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "bad",
								Expression: "1 + 'a'", // Invalid CEL expression
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "object.spec.containers.size() > 0",
							},
						},
					},
				},
			},
		}

		rec := newReconciler(
			compiler.NewCompiler(),
			&fakeClient{policy: vp},
			nil, false,
		)

		name := types.NamespacedName{Name: "test-policy"}
		// Pre-populate the cache as if it was successfully compiled before
		rec.policies = map[string][]Policy{
			name.String(): {{}},
		}

		// Reconcile - it should fail compilation and remove it from the cache
		res, err := rec.Reconcile(ctx, reconcile.Request{NamespacedName: name})
		assert.NoError(t, err)
		assert.Equal(t, reconcile.Result{}, res)

		// Ensure cache is cleared
		_, exists := rec.policies[name.String()]
		assert.False(t, exists)
	})
}
