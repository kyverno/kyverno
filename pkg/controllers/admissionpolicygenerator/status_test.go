package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestUpdatePolicyStatusReturnsConflict(t *testing.T) {
	t.Parallel()
	policies := map[string]engineapi.GenericPolicy{
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

// TestReconcileDeletesAdmissionPoliciesForPoliciesOwnedByOtherControllers verifies that, when a
// ValidatingPolicy changes owners, the admission policy and binding Kyverno generated for it are
// deleted, that a same-named object Kyverno didn't generate is kept, that a failed delete is returned
// for retry, and that no status is ever written.
func TestReconcileDeletesAdmissionPoliciesForPoliciesOwnedByOtherControllers(t *testing.T) {
	t.Parallel()
	const policyUID = types.UID("policy-uid")
	generated := []metav1.OwnerReference{{Kind: "ValidatingPolicy", Name: "policy", UID: policyUID}}
	tests := []struct {
		name string
		// owners of the cached ValidatingAdmissionPolicy and binding
		owners []metav1.OwnerReference
		// inClient is false when the objects are cached but already gone from the API server
		inClient bool
		// failDelete is the resource whose first delete fails
		failDelete  string
		wantDeleted bool
	}{
		{name: "generated objects are deleted", owners: generated, inClient: true, wantDeleted: true},
		{name: "objects generated for another policy are kept", owners: []metav1.OwnerReference{{Kind: "ValidatingPolicy", Name: "policy", UID: "other-uid"}}, inClient: true},
		{name: "objects without owners are kept", inClient: true},
		{name: "a NotFound delete after a cached lookup succeeds", owners: generated},
		{name: "a failed policy delete is retried", owners: generated, inClient: true, failDelete: "validatingadmissionpolicies", wantDeleted: true},
		{name: "a failed binding delete is retried", owners: generated, inClient: true, failDelete: "validatingadmissionpolicybindings", wantDeleted: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			vpol := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", UID: policyUID},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
				},
			}
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-policy", UID: "vap-uid", OwnerReferences: tt.owners}}
			binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "vpol-policy-binding", UID: "binding-uid", OwnerReferences: tt.owners}}
			client := kubefake.NewSimpleClientset()
			if tt.inClient {
				client = kubefake.NewSimpleClientset(vap, binding)
			}
			kyvernoClient := versionedfake.NewSimpleClientset(vpol)
			c := &controller{
				client:           client,
				kyvernoClient:    kyvernoClient,
				vpolLister:       policiesv1beta1listers.NewValidatingPolicyLister(newIndexer(t, vpol)),
				vapLister:        admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(newIndexer(t, vap)),
				vapbindingLister: admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(newIndexer(t, binding)),
			}

			if tt.failDelete != "" {
				failing := true
				client.PrependReactor("delete", tt.failDelete, func(clienttesting.Action) (bool, runtime.Object, error) {
					if failing {
						return true, nil, errors.New("delete failed")
					}
					return false, nil, nil
				})
				require.Error(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/policy", "", "policy"), "a failed delete must be returned so the workqueue retries")
				failing = false
			}
			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/policy", "", "policy"))

			if tt.inClient {
				_, vapErr := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vap.Name, metav1.GetOptions{})
				_, bindingErr := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
				assert.Equal(t, tt.wantDeleted, apierrors.IsNotFound(vapErr), "ValidatingAdmissionPolicy deleted")
				assert.Equal(t, tt.wantDeleted, apierrors.IsNotFound(bindingErr), "binding deleted")
			}
			assert.Empty(t, statusUpdates(kyvernoClient.Actions()), "the controller must not write status on a policy it doesn't own")
		})
	}
}

// TestReconcileDeletesMutatingAdmissionPoliciesForPoliciesOwnedByOtherControllers verifies the same
// cleanup for a MutatingPolicy, and that a same-named object Kyverno didn't generate is kept.
func TestReconcileDeletesMutatingAdmissionPoliciesForPoliciesOwnedByOtherControllers(t *testing.T) {
	t.Parallel()
	const policyUID = types.UID("policy-uid")
	tests := []struct {
		name        string
		owners      []metav1.OwnerReference
		wantDeleted bool
	}{
		{name: "generated objects are deleted", owners: []metav1.OwnerReference{{Kind: "MutatingPolicy", Name: "policy", UID: policyUID}}, wantDeleted: true},
		{name: "objects not generated for the policy are kept"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mpol := &policiesv1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "policy", UID: policyUID},
				Spec: policiesv1beta1.MutatingPolicySpec{
					EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: "HTTP"},
				},
			}
			mapol := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mpol-policy", OwnerReferences: tt.owners}}
			binding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "mpol-policy-binding", OwnerReferences: tt.owners}}
			kyvernoClient := versionedfake.NewSimpleClientset(mpol)
			client := kubefake.NewSimpleClientset(mapol, binding)
			c := &controller{
				client:             client,
				kyvernoClient:      kyvernoClient,
				mpolLister:         policiesv1beta1listers.NewMutatingPolicyLister(newIndexer(t, mpol)),
				mapV1Lister:        admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(newIndexer(t, mapol)),
				mapbindingV1Lister: admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(newIndexer(t, binding)),
			}

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/policy", "", "policy"))
			_, mapErr := client.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, mapol.Name, metav1.GetOptions{})
			_, bindingErr := client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
			assert.Equal(t, tt.wantDeleted, apierrors.IsNotFound(mapErr), "MutatingAdmissionPolicy deleted")
			assert.Equal(t, tt.wantDeleted, apierrors.IsNotFound(bindingErr), "binding deleted")
			assert.Empty(t, statusUpdates(kyvernoClient.Actions()), "the controller must not write status on a policy it doesn't own")
		})
	}
}

func newIndexer(t *testing.T, objects ...any) cache.Indexer {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	for _, obj := range objects {
		require.NoError(t, indexer.Add(obj))
	}
	return indexer
}

// statusUpdates returns the status subresource updates among the recorded client actions.
func statusUpdates(actions []clienttesting.Action) []clienttesting.Action {
	var out []clienttesting.Action
	for _, action := range actions {
		if action.GetVerb() == "update" && action.GetSubresource() == "status" {
			out = append(out, action)
		}
	}
	return out
}

// TestReconcileSkipsPoliciesOwnedByOtherControllers verifies that no admission policy is generated,
// and no status is written, for a policy whose evaluation mode belongs to another controller.
func TestReconcileSkipsPoliciesOwnedByOtherControllers(t *testing.T) {
	t.Parallel()
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: "Envoy"},
		},
	}
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration:    mapGenEnabled(),
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: "HTTP"},
		},
	}
	tests := []struct {
		name   string
		key    string
		object runtime.Object
		setup  func(c *controller, indexer cache.Indexer)
	}{
		{
			name:   "ValidatingPolicy",
			key:    "ValidatingPolicy/policy",
			object: vpol,
			setup: func(c *controller, indexer cache.Indexer) {
				c.vpolLister = policiesv1beta1listers.NewValidatingPolicyLister(indexer)
			},
		},
		{
			name:   "MutatingPolicy",
			key:    "MutatingPolicy/policy",
			object: mpol,
			setup: func(c *controller, indexer cache.Indexer) {
				c.mpolLister = policiesv1beta1listers.NewMutatingPolicyLister(indexer)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client := versionedfake.NewSimpleClientset(tt.object)
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			require.NoError(t, indexer.Add(tt.object))
			c := &controller{kyvernoClient: client}
			tt.setup(c, indexer)

			require.NoError(t, c.reconcile(context.Background(), logr.Discard(), tt.key, "", "policy"))
			assert.Empty(t, statusUpdates(client.Actions()), "the controller must not write status on a policy it doesn't own")
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
