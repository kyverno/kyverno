package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestUpdatePolicyStatusReturnsConflict(t *testing.T) {
	t.Parallel()
	policies := map[string]engineapi.GenericPolicy{
		"cluster": engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		}),
		"validating": engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		}),
		"mutating": engineapi.NewMutatingPolicy(&policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		}),
	}
	for name, policy := range policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := versionedfake.NewSimpleClientset()
			client.PrependReactor("update", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
				require.Equal(t, "status", action.GetSubresource())
				return true, nil, apierrors.NewConflict(action.GetResource().GroupResource(), "policy", nil)
			})
			c := &controller{kyvernoClient: client}
			assert.True(t, apierrors.IsConflict(c.updatePolicyStatus(context.Background(), policy, true, "")))
		})
	}
}

func TestReconcileReturnsStatusConflictForRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	policy := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration: mapGenEnabled(),
		},
	}
	client := versionedfake.NewSimpleClientset(policy)
	conflict := true
	client.PrependReactor("update", "mutatingpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if conflict {
			conflict = false
			return true, nil, apierrors.NewConflict(schema.GroupResource{
				Group: "policies.kyverno.io", Resource: "mutatingpolicies",
			}, policy.Name, nil)
		}
		return false, nil, nil
	})
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(policy))
	c := &controller{
		kyvernoClient: client,
		mpolLister:    policiesv1beta1listers.NewMutatingPolicyLister(indexer),
	}

	err := c.reconcile(ctx, logr.Discard(), "MutatingPolicy/policy", "", policy.Name)
	require.True(t, apierrors.IsConflict(err), "the workqueue must retry failed status writes")
	require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/policy", "", policy.Name))
	updated, err := client.PoliciesV1beta1().MutatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "no MutatingAdmissionPolicy API/informers")
	assert.Empty(t, policy.Status.GetConditionStatus().Message, "the informer object must not be modified")
}
