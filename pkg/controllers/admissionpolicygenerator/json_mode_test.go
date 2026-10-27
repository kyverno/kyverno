package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

// A JSON mode policy has no Kubernetes admission equivalent, so no native
// MutatingAdmissionPolicy is generated for it even when generation is requested.
func TestMapGenerationSkipReason_JSONMode(t *testing.T) {
	policy := &policiesv1beta1.MutatingPolicy{
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration:    mapGenEnabled(),
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
	}
	reason, err := mapGenerationSkipReason(policy)
	assert.NoError(t, err)
	assert.Equal(t, "skip generating MutatingAdmissionPolicy: JSON evaluation mode has no admission equivalent.", reason)

	policy.Spec.EvaluationConfiguration.Mode = policieskyvernoio.EvaluationModeKubernetes
	reason, err = mapGenerationSkipReason(policy)
	assert.NoError(t, err)
	assert.Empty(t, reason)
}

// Switching a policy with a generated MutatingAdmissionPolicy to JSON mode
// deletes the generated MAP and binding and clears status.generated, so the
// policy is no longer enforced at admission. Cleanup must also happen when the
// controller lacks the RBAC needed to generate admission policies, and when the
// generation toggle (off by default) is off and the policy no longer opts in.
func TestHandleMAPGeneration_JSONModeDeletesGeneratedMAP(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		authChecker  checker.AuthChecker
		viaReconcile bool
	}{
		"permissive":                       {authChecker: permissiveAuthChecker{}},
		"generation denied":                {authChecker: denyingAuthChecker{}},
		"toggle off without policy opt-in": {authChecker: permissiveAuthChecker{}, viaReconcile: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testJSONModeDeletesGeneratedMAP(t, tc.authChecker, tc.viaReconcile)
		})
	}
}

// denyingAuthChecker denies every permission check.
type denyingAuthChecker struct{}

func (denyingAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: false}, nil
}

func testJSONModeDeletesGeneratedMAP(t *testing.T, authChecker checker.AuthChecker, viaReconcile bool) {
	ctx := context.Background()
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration:    mapGenEnabled(),
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
		Status: policiesv1beta1.MutatingPolicyStatus{Generated: true},
	}
	mapName := "mpol-" + mpol.Name
	bindingName := constructBindingName(mapName)
	existingMAP := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: mapName, ResourceVersion: "1"}}
	existingBinding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName, ResourceVersion: "1"}}

	kubeClient := kubefake.NewSimpleClientset(existingMAP, existingBinding)
	kyvernoClient := versionedfake.NewSimpleClientset(mpol)
	mapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, mapIndexer.Add(existingMAP))
	require.NoError(t, bindingIndexer.Add(existingBinding))
	c := &controller{
		client:             kubeClient,
		kyvernoClient:      kyvernoClient,
		checker:            authChecker,
		mapV1Lister:        admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapIndexer),
		mapbindingV1Lister: admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	if viaReconcile {
		mpol.Spec.AutogenConfiguration = nil
		mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		require.NoError(t, mpolIndexer.Add(mpol))
		c.mpolLister = policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer)
		require.False(t, toggle.FromContext(ctx).GenerateMutatingAdmissionPolicy())
		require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))
	} else {
		require.NoError(t, c.handleMAPGeneration(ctx, mpol))
	}

	_, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, mapName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "generated MAP must be deleted")
	_, err = kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err), "generated MAP binding must be deleted")
	updated, err := kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(ctx, mpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "JSON evaluation mode")
}

// A failing MAP delete must surface the error (so the item is requeued) and must
// not clear status.generated while the generated MAP may still be enforced.
func TestHandleMAPGeneration_JSONModeDeleteFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "json-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
		Status: policiesv1beta1.MutatingPolicyStatus{Generated: true},
	}
	mapName := "mpol-" + mpol.Name
	bindingName := constructBindingName(mapName)
	existingMAP := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: mapName, ResourceVersion: "1"}}
	existingBinding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName, ResourceVersion: "1"}}

	kubeClient := kubefake.NewSimpleClientset(existingMAP, existingBinding)
	deleteErr := errors.New("delete failed")
	kubeClient.PrependReactor("delete", "mutatingadmissionpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, deleteErr
	})
	kyvernoClient := versionedfake.NewSimpleClientset(mpol)
	mapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, mapIndexer.Add(existingMAP))
	require.NoError(t, bindingIndexer.Add(existingBinding))
	c := &controller{
		client:             kubeClient,
		kyvernoClient:      kyvernoClient,
		checker:            permissiveAuthChecker{},
		mapV1Lister:        admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapIndexer),
		mapbindingV1Lister: admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(bindingIndexer),
	}

	require.ErrorIs(t, c.handleMAPGeneration(ctx, mpol), deleteErr)
	_, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, mapName, metav1.GetOptions{})
	require.NoError(t, err, "MAP must remain after a failed delete")
	updated, err := kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(ctx, mpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.True(t, updated.Status.Generated, "status.generated must not be cleared on failure")
}

type failingMAPIndexer struct {
	cache.Indexer
	err error
}

func (i failingMAPIndexer) GetByKey(string) (interface{}, bool, error) {
	return nil, false, i.err
}

func TestHandleMAPGeneration_JSONModeCleanupErrors(t *testing.T) {
	t.Parallel()
	for _, version := range []admissionpolicy.MutatingAdmissionPolicyVersion{
		admissionpolicy.MutatingAdmissionPolicyVersionV1,
		admissionpolicy.MutatingAdmissionPolicyVersionV1beta1,
		admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1,
	} {
		t.Run(string(version), func(t *testing.T) {
			t.Parallel()
			for _, scenario := range []string{"policy lookup error", "binding lookup error", "binding delete error", "already absent", "stale cache"} {
				t.Run(scenario, func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()
					mpol := &policiesv1beta1.MutatingPolicy{
						ObjectMeta: metav1.ObjectMeta{Name: "json-policy"},
						Spec: policiesv1beta1.MutatingPolicySpec{
							EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
						},
						Status: policiesv1beta1.MutatingPolicyStatus{Generated: true},
					}
					mapMeta := metav1.ObjectMeta{Name: "mpol-" + mpol.Name}
					bindingMeta := metav1.ObjectMeta{Name: constructBindingName(mapMeta.Name)}
					var existingMAP, existingBinding runtime.Object
					switch version {
					case admissionpolicy.MutatingAdmissionPolicyVersionV1:
						existingMAP = &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: mapMeta}
						existingBinding = &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: bindingMeta}
					case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
						existingMAP = &admissionregistrationv1beta1.MutatingAdmissionPolicy{ObjectMeta: mapMeta}
						existingBinding = &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{ObjectMeta: bindingMeta}
					case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
						existingMAP = &admissionregistrationv1alpha1.MutatingAdmissionPolicy{ObjectMeta: mapMeta}
						existingBinding = &admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{ObjectMeta: bindingMeta}
					}
					kubeClient := kubefake.NewSimpleClientset()
					if scenario != "stale cache" && scenario != "already absent" {
						kubeClient = kubefake.NewSimpleClientset(existingMAP, existingBinding)
					}
					kyvernoClient := versionedfake.NewSimpleClientset(mpol)
					mapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
					bindingIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
					if scenario != "already absent" {
						require.NoError(t, mapIndexer.Add(existingMAP))
						require.NoError(t, bindingIndexer.Add(existingBinding))
					}
					var policyCache, bindingCache cache.Indexer = mapIndexer, bindingIndexer
					wantErr := errors.New("cleanup unavailable")
					switch scenario {
					case "policy lookup error":
						policyCache = failingMAPIndexer{Indexer: mapIndexer, err: wantErr}
					case "binding lookup error":
						bindingCache = failingMAPIndexer{Indexer: bindingIndexer, err: wantErr}
					case "binding delete error":
						kubeClient.PrependReactor("delete", "mutatingadmissionpolicybindings", func(k8stesting.Action) (bool, runtime.Object, error) {
							return true, nil, wantErr
						})
					}
					c := &controller{client: kubeClient, kyvernoClient: kyvernoClient}
					switch version {
					case admissionpolicy.MutatingAdmissionPolicyVersionV1:
						c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(policyCache)
						c.mapbindingV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(bindingCache)
					case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
						c.mapBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyLister(policyCache)
						c.mapbindingBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyBindingLister(bindingCache)
					case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
						c.mapAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyLister(policyCache)
						c.mapbindingAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyBindingLister(bindingCache)
					}
					err := c.handleMAPGeneration(ctx, mpol)
					failed := scenario != "already absent" && scenario != "stale cache"
					if failed {
						require.ErrorIs(t, err, wantErr)
						require.Empty(t, kyvernoClient.Actions(), "status must not be updated on cleanup failure")
					} else {
						require.NoError(t, err)
					}
					updated, err := kyvernoClient.PoliciesV1beta1().MutatingPolicies().Get(ctx, mpol.Name, metav1.GetOptions{})
					require.NoError(t, err)
					assert.Equal(t, failed, updated.Status.Generated)
					if scenario == "policy lookup error" || scenario == "binding lookup error" {
						assert.Empty(t, kubeClient.Actions(), "lookup errors must be checked before deleting either resource")
					}
				})
			}
		})
	}
}
