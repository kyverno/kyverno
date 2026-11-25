package admissionpolicygenerator

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kubefake "k8s.io/client-go/kubernetes/fake"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	admissionregistrationv1alpha1listers "k8s.io/client-go/listers/admissionregistration/v1alpha1"
	admissionregistrationv1beta1listers "k8s.io/client-go/listers/admissionregistration/v1beta1"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
)

const (
	testVpolName       = "disallow-privilege-escalation"
	testVAPName        = "vpol-disallow-privilege-escalation"
	testVAPBindingName = "vpol-disallow-privilege-escalation-binding"
	testMpolName       = "add-team-label"
	testMAPName        = "mpol-add-team-label"
	testMAPBindingName = "mpol-add-team-label-binding"
	testVpolUID        = types.UID("6f1c2a3e-vpol")
	testMpolUID        = types.UID("9b4d7e21-mpol")
)

// generatedBy is the owner reference the generator sets on each native admission policy and
// binding it writes for a policy.
func generatedBy(kind, name string, uid types.UID) []metav1.OwnerReference {
	return []metav1.OwnerReference{{APIVersion: policiesv1beta1.GroupVersion.String(), Kind: kind, Name: name, UID: uid}}
}

// failFirstDelete makes the first delete of the resource fail, as a transient API server error
// would, and lets later deletes through.
func failFirstDelete(client *kubefake.Clientset, resource string) {
	failed := false
	client.PrependReactor("delete", resource, func(clienttesting.Action) (bool, runtime.Object, error) {
		if failed {
			return false, nil, nil
		}
		failed = true
		return true, nil, unavailableError()
	})
}

// The workqueue retries a failed reconcile with the policy as the informer has it by then, which
// includes the status the generator wrote. vpolListerWith and mpolListerWith stand in for that.
func vpolListerWith(t *testing.T, vpol *policiesv1beta1.ValidatingPolicy) policiesv1beta1listers.ValidatingPolicyLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(vpol))
	return policiesv1beta1listers.NewValidatingPolicyLister(indexer)
}

func mpolListerWith(t *testing.T, mpol *policiesv1beta1.MutatingPolicy) policiesv1beta1listers.MutatingPolicyLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(mpol))
	return policiesv1beta1listers.NewMutatingPolicyLister(indexer)
}

// withOwners replaces the owner references of the objects. Without any, they stand in for objects
// created by hand that only share the generated names.
func withOwners(refs []metav1.OwnerReference, objs ...runtime.Object) []runtime.Object {
	for _, obj := range objs {
		obj.(metav1.Object).SetOwnerReferences(refs)
	}
	return objs
}

var (
	vapGroupKind = schema.GroupKind{Group: "admissionregistration.k8s.io", Kind: "ValidatingAdmissionPolicy"}
	mapGroupKind = schema.GroupKind{Group: "admissionregistration.k8s.io", Kind: "MutatingAdmissionPolicy"}
)

// rejectedExpressionError is what the API server returns when a PolicyException's matchCondition
// calls a Kyverno-only CEL library, which a native admission policy cannot compile.
func rejectedExpressionError(gk schema.GroupKind, name string) error {
	return apierrors.NewInvalid(gk, name, field.ErrorList{
		field.Invalid(field.NewPath("spec", "matchConditions").Index(0).Child("expression"),
			"!('team' in resource.Get('v1', 'namespaces', '', object.metadata.namespace).metadata.labels)",
			"compilation failed: ERROR: <input>:1:13: undeclared reference to 'resource'"),
	})
}

func unavailableError() error {
	return apierrors.NewServiceUnavailable("etcd leader changed")
}

// generatedVpol is a ValidatingPolicy whose ValidatingAdmissionPolicy was generated earlier, so
// Kyverno's webhooks and CEL engine currently leave admission to the native policy.
func generatedVpol() *policiesv1beta1.ValidatingPolicy {
	vpol := disallowPrivilegeEscalationVpol(nil)
	// autogen is on by default for pod-shaped policies, so an empty controllers list turns it off
	vpol.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
	vpol.UID = testVpolUID
	vpol.Status.Generated = true
	return vpol
}

// staleVAP and staleVAPBinding stand in for objects generated from an older revision of the
// policy, so regenerating them has to update them.
func staleVAP() *admissionregistrationv1.ValidatingAdmissionPolicy {
	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: testVAPName, ResourceVersion: "1", OwnerReferences: generatedBy("ValidatingPolicy", testVpolName, testVpolUID)},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
}

func staleVAPBinding() *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: testVAPBindingName, ResourceVersion: "1", OwnerReferences: generatedBy("ValidatingPolicy", testVpolName, testVpolUID)},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        testVAPName,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
		},
	}
}

func getVpol(t *testing.T, client *versionedfake.Clientset, name string) *policiesv1beta1.ValidatingPolicy {
	t.Helper()
	vpol, err := client.PoliciesV1beta1().ValidatingPolicies().Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return vpol
}

// TestHandleVAPGeneration_FailureHandsAdmissionBackToKyverno covers a policy whose
// ValidatingAdmissionPolicy was generated before and then fails to regenerate. While
// status.generated is true, Kyverno's webhooks and CEL engine skip the policy, so the generator has
// to clear it, or the stale (or missing) native policy stays the only enforcer.
func TestHandleVAPGeneration_FailureHandsAdmissionBackToKyverno(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		existing func() []runtime.Object
		verb     string
		resource string
		err      error
		// wantVAP is set when a ValidatingAdmissionPolicy existed before the failure
		wantVAP bool
	}{{
		name:     "update of the existing ValidatingAdmissionPolicy is rejected",
		existing: func() []runtime.Object { return []runtime.Object{staleVAP(), staleVAPBinding()} },
		verb:     "update",
		resource: "validatingadmissionpolicies",
		err:      rejectedExpressionError(vapGroupKind, testVAPName),
		wantVAP:  true,
	}, {
		name:     "ValidatingAdmissionPolicy deleted out of band and re-creating it fails",
		existing: func() []runtime.Object { return []runtime.Object{staleVAPBinding()} },
		verb:     "create",
		resource: "validatingadmissionpolicies",
		err:      unavailableError(),
	}, {
		name:     "update of the existing binding fails",
		existing: func() []runtime.Object { return []runtime.Object{staleVAP(), staleVAPBinding()} },
		verb:     "update",
		resource: "validatingadmissionpolicybindings",
		err:      unavailableError(),
		wantVAP:  true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			vpol := generatedVpol()
			c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, tt.existing()...)
			kubeClient.PrependReactor(tt.verb, tt.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.err
			})

			err := c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol))
			require.Error(t, err, "the generation error must be returned so the workqueue retries")

			updated := getVpol(t, kyvernoClient, vpol.Name)
			assert.False(t, updated.Status.Generated, "Kyverno must evaluate the policy again")
			assert.Contains(t, updated.Status.GetConditionStatus().Message, "failed to generate ValidatingAdmissionPolicy")
			if tt.wantVAP {
				_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, testVAPName, metav1.GetOptions{})
				assert.NoError(t, vapErr, "the last generated ValidatingAdmissionPolicy must be left in place")
			}
		})
	}
}

// TestHandleVAPGeneration_FallbackReturnsStatusWriteError checks that a failed status write in the
// fallback is returned with the generation error, so the workqueue retries with a fresh object
// (pkg/controllers/AGENTS.md) instead of the policy keeping status.generated=true.
func TestHandleVAPGeneration_FallbackReturnsStatusWriteError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vpol := generatedVpol()
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())
	kubeClient.PrependReactor("update", "validatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, unavailableError()
	})
	kyvernoClient.PrependReactor("update", "validatingpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(action.GetResource().GroupResource(), vpol.Name, nil)
	})

	err := c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol))
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "the status conflict must reach the workqueue: %v", err)
	// the generation error is wrapped with %v upstream of handleGenerationError, so check its text
	assert.ErrorContains(t, err, "failed to update validatingadmissionpolicy")
}

// TestHandleVAPGeneration_RecoversAfterTransientFailure checks that the fallback is not sticky:
// once the ValidatingAdmissionPolicy can be updated again, admission goes back to it.
func TestHandleVAPGeneration_RecoversAfterTransientFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vpol := generatedVpol()
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())
	failing := true
	kubeClient.PrependReactor("update", "validatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		if failing {
			return true, nil, unavailableError()
		}
		return false, nil, nil
	})

	require.Error(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))
	fellBack := getVpol(t, kyvernoClient, vpol.Name)
	require.False(t, fellBack.Status.Generated)

	failing = false
	require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(fellBack)))
	recovered := getVpol(t, kyvernoClient, vpol.Name)
	assert.True(t, recovered.Status.Generated)
	assert.Empty(t, recovered.Status.GetConditionStatus().Message)
}

// TestHandleVAPGeneration_FailedDeleteHandsAdmissionBackToKyverno covers turning VAP generation off
// for a policy whose ValidatingAdmissionPolicy cannot be deleted right away. Kyverno has to take
// admission back before the delete, so a failed delete leaves the old native policy as an extra
// enforcer instead of the only one.
func TestHandleVAPGeneration_FailedDeleteHandsAdmissionBackToKyverno(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vpol := generatedVpol()
	vpol.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())
	kubeClient.PrependReactor("delete", "validatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, unavailableError()
	})

	require.Error(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))
	updated := getVpol(t, kyvernoClient, vpol.Name)
	assert.False(t, updated.Status.Generated)
	assert.Equal(t, "skip generating ValidatingAdmissionPolicy: not enabled.", updated.Status.GetConditionStatus().Message)
}

// vapFlagOff returns a context with the generateValidatingAdmissionPolicy flag turned off.
func vapFlagOff() context.Context {
	return toggle.NewContext(context.Background(), fakeVAPToggle{Toggles: toggle.FromContext(context.Background())})
}

// optedOutVpol is a ValidatingPolicy that turned VAP generation off after its
// ValidatingAdmissionPolicy was generated.
func optedOutVpol() *policiesv1beta1.ValidatingPolicy {
	vpol := generatedVpol()
	vpol.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
	return vpol
}

// TestReconcile_VAPFlagOffRemovesPolicyThatOptedOut covers a policy that turns VAP generation off
// while the generateValidatingAdmissionPolicy flag is off. Kyverno takes admission back and the
// ValidatingAdmissionPolicy generated earlier is removed.
func TestReconcile_VAPFlagOffRemovesPolicyThatOptedOut(t *testing.T) {
	t.Parallel()
	assertVAPRemoved := func(t *testing.T, ctx context.Context, kubeClient *kubefake.Clientset) {
		t.Helper()
		_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, testVAPName, metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(vapErr), "the ValidatingAdmissionPolicy must be removed: got err=%v", vapErr)
		_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, testVAPBindingName, metav1.GetOptions{})
		assert.True(t, apierrors.IsNotFound(bindingErr), "the binding must be removed: got err=%v", bindingErr)
	}

	t.Run("generated policy", func(t *testing.T) {
		t.Parallel()
		ctx := vapFlagOff()
		vpol := optedOutVpol()
		c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())

		require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))

		assertVAPRemoved(t, ctx, kubeClient)
		updated := getVpol(t, kyvernoClient, vpol.Name)
		assert.False(t, updated.Status.Generated)
		assert.Equal(t, "skip generating ValidatingAdmissionPolicy: not enabled.", updated.Status.GetConditionStatus().Message)
	})

	// The status is cleared before the delete, so the retry of a failed delete reads
	// generated=false and still has to find and remove the ValidatingAdmissionPolicy.
	t.Run("retry after a failed delete", func(t *testing.T) {
		t.Parallel()
		ctx := vapFlagOff()
		vpol := optedOutVpol()
		c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())
		failFirstDelete(kubeClient, "validatingadmissionpolicies")
		queue := &delayRecorder{delays: map[any]time.Duration{}}
		c.queue = queue

		require.Error(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))
		// Status-only changes never requeue the policy, so the retry has to outlive the workqueue's budget.
		assert.Equal(t, 10*time.Second, queue.delays["ValidatingPolicy/"+vpol.Name], "a delayed retry must be scheduled")
		retried := getVpol(t, kyvernoClient, vpol.Name)
		require.False(t, retried.Status.Generated)
		c.vpolLister = vpolListerWith(t, retried)
		require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))

		assertVAPRemoved(t, ctx, kubeClient)
	})
}

// TestReconcile_VAPFlagOffLeavesObjectsKyvernoDidNotGenerate covers objects that only share the
// generated names: ones created by hand, and ones left over from a deleted policy of the same
// name. Removing what was generated for the policy leaves them alone.
func TestReconcile_VAPFlagOffLeavesObjectsKyvernoDidNotGenerate(t *testing.T) {
	t.Parallel()
	leftOver := generatedBy("ValidatingPolicy", testVpolName, "uid-of-a-deleted-policy")
	for _, tc := range []struct {
		name                 string
		existing             []runtime.Object
		keepVAP, keepBinding bool
	}{
		{"created by hand", withOwners(nil, staleVAP(), staleVAPBinding()), true, true},
		{"left over from a deleted policy", withOwners(leftOver, staleVAP(), staleVAPBinding()), true, true},
		{"only the binding generated", append(withOwners(nil, staleVAP()), staleVAPBinding()), true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := vapFlagOff()
			vpol := optedOutVpol()
			c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, tc.existing...)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name))

			_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, testVAPName, metav1.GetOptions{})
			assert.Equal(t, tc.keepVAP, vapErr == nil, "ValidatingAdmissionPolicy kept, err=%v", vapErr)
			_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, testVAPBindingName, metav1.GetOptions{})
			assert.Equal(t, tc.keepBinding, bindingErr == nil, "binding kept, err=%v", bindingErr)
			assert.False(t, getVpol(t, kyvernoClient, vpol.Name).Status.Generated)
		})
	}
}

// enforceUIDPrecondition makes the fake API server check the UID precondition of a delete, as a
// real one does: the fake object tracker ignores preconditions.
func enforceUIDPrecondition(client *kubefake.Clientset, resource string) {
	client.PrependReactor("delete", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
		del := action.(clienttesting.DeleteAction)
		stored, err := client.Tracker().Get(del.GetResource(), del.GetNamespace(), del.GetName())
		if p := del.GetDeleteOptions().Preconditions; err == nil && p != nil && p.UID != nil && *p.UID != stored.(metav1.Object).GetUID() {
			return true, nil, apierrors.NewConflict(del.GetResource().GroupResource(), del.GetName(), nil)
		}
		return false, nil, nil
	})
}

// TestReconcile_VAPFlagOffLeavesObjectReplacedAfterCacheRead covers a ValidatingAdmissionPolicy
// that was replaced by one Kyverno didn't generate after the informer cache was read. The delete
// carries the cached UID as a precondition, so the API server refuses it and the replacement stays.
func TestReconcile_VAPFlagOffLeavesObjectReplacedAfterCacheRead(t *testing.T) {
	t.Parallel()
	ctx := vapFlagOff()
	vpol := optedOutVpol()
	replacement := staleVAP()
	replacement.UID, replacement.OwnerReferences = "uid-of-the-replacement", nil
	c, kubeClient, _ := newVAPTestController(t, vpol, replacement)
	cached := staleVAP()
	cached.UID = "uid-of-the-generated-vap"
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(cached))
	c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(indexer)
	enforceUIDPrecondition(kubeClient, "validatingadmissionpolicies")

	err := c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+vpol.Name, "", vpol.Name)

	assert.True(t, apierrors.IsConflict(err), "the API server must refuse the delete: %v", err)
	_, getErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, testVAPName, metav1.GetOptions{})
	assert.NoError(t, getErr, "the replacement must not be deleted")
}

// generatedMpol is a MutatingPolicy whose MutatingAdmissionPolicy was generated earlier.
func generatedMpol() *policiesv1beta1.MutatingPolicy {
	return &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: testMpolName, UID: testMpolUID},
		Spec: policiesv1beta1.MutatingPolicySpec{
			AutogenConfiguration: mapGenEnabled(),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"configmaps"},
						},
					},
				}},
			},
		},
		Status: policiesv1beta1.MutatingPolicyStatus{Generated: true},
	}
}

// mapVersionCase is one MutatingAdmissionPolicy API version the generator can write.
type mapVersionCase struct {
	version admissionpolicy.MutatingAdmissionPolicyVersion
	// stale returns a MutatingAdmissionPolicy and binding of this version generated from an older
	// revision of the policy, so regenerating them has to update them.
	stale         func() []runtime.Object
	getMAP        func(ctx context.Context, client *kubefake.Clientset) error
	getMAPBinding func(ctx context.Context, client *kubefake.Clientset) error
}

func mapVersionCases() []mapVersionCase {
	return []mapVersionCase{{
		version: admissionpolicy.MutatingAdmissionPolicyVersionV1,
		stale: func() []runtime.Object {
			return []runtime.Object{
				&admissionregistrationv1.MutatingAdmissionPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec: admissionregistrationv1.MutatingAdmissionPolicySpec{
						Variables: []admissionregistrationv1.Variable{{Name: "stale", Expression: "true"}},
					},
				},
				&admissionregistrationv1.MutatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPBindingName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec:       admissionregistrationv1.MutatingAdmissionPolicyBindingSpec{PolicyName: "stale"},
				},
			}
		},
		getMAP: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, testMAPName, metav1.GetOptions{})
			return err
		},
		getMAPBinding: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1().MutatingAdmissionPolicyBindings().Get(ctx, testMAPBindingName, metav1.GetOptions{})
			return err
		},
	}, {
		version: admissionpolicy.MutatingAdmissionPolicyVersionV1beta1,
		stale: func() []runtime.Object {
			return []runtime.Object{
				&admissionregistrationv1beta1.MutatingAdmissionPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec: admissionregistrationv1beta1.MutatingAdmissionPolicySpec{
						Variables: []admissionregistrationv1beta1.Variable{{Name: "stale", Expression: "true"}},
					},
				},
				&admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPBindingName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec:       admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{PolicyName: "stale"},
				},
			}
		},
		getMAP: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1beta1().MutatingAdmissionPolicies().Get(ctx, testMAPName, metav1.GetOptions{})
			return err
		},
		getMAPBinding: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1beta1().MutatingAdmissionPolicyBindings().Get(ctx, testMAPBindingName, metav1.GetOptions{})
			return err
		},
	}, {
		version: admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1,
		stale: func() []runtime.Object {
			return []runtime.Object{
				&admissionregistrationv1alpha1.MutatingAdmissionPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec: admissionregistrationv1alpha1.MutatingAdmissionPolicySpec{
						Variables: []admissionregistrationv1alpha1.Variable{{Name: "stale", Expression: "true"}},
					},
				},
				&admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: testMAPBindingName, ResourceVersion: "1", OwnerReferences: generatedBy("MutatingPolicy", testMpolName, testMpolUID)},
					Spec:       admissionregistrationv1alpha1.MutatingAdmissionPolicyBindingSpec{PolicyName: "stale"},
				},
			}
		},
		getMAP: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicies().Get(ctx, testMAPName, metav1.GetOptions{})
			return err
		},
		getMAPBinding: func(ctx context.Context, client *kubefake.Clientset) error {
			_, err := client.AdmissionregistrationV1alpha1().MutatingAdmissionPolicyBindings().Get(ctx, testMAPBindingName, metav1.GetOptions{})
			return err
		},
	}}
}

// newVersionedMAPTestController builds a controller that generates MutatingAdmissionPolicies of the given
// API version, backed by fake clientsets. The listers hold only the objects passed as existing.
func newVersionedMAPTestController(t *testing.T, version admissionpolicy.MutatingAdmissionPolicyVersion, mpol *policiesv1beta1.MutatingPolicy, existing ...runtime.Object) (*controller, *kubefake.Clientset, *versionedfake.Clientset) {
	t.Helper()
	kubeClient := kubefake.NewSimpleClientset(existing...)
	kyvernoClient := versionedfake.NewSimpleClientset(mpol)
	newIndexer := func() cache.Indexer { return cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}) }
	mpolIndexer, mapIndexer, mapBindingIndexer := newIndexer(), newIndexer(), newIndexer()
	require.NoError(t, mpolIndexer.Add(mpol))
	for _, obj := range existing {
		switch obj.(type) {
		case *admissionregistrationv1.MutatingAdmissionPolicy, *admissionregistrationv1beta1.MutatingAdmissionPolicy, *admissionregistrationv1alpha1.MutatingAdmissionPolicy:
			require.NoError(t, mapIndexer.Add(obj))
		case *admissionregistrationv1.MutatingAdmissionPolicyBinding, *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding:
			require.NoError(t, mapBindingIndexer.Add(obj))
		}
	}
	c := &controller{
		client:         kubeClient,
		kyvernoClient:  kyvernoClient,
		eventGen:       event.NewFake(),
		checker:        permissiveAuthChecker{},
		mpolLister:     policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
		celpolexLister: policiesv1beta1listers.NewPolicyExceptionLister(newIndexer()),
	}
	switch version {
	case admissionpolicy.MutatingAdmissionPolicyVersionV1:
		c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapIndexer)
		c.mapbindingV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyBindingLister(mapBindingIndexer)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1beta1:
		c.mapBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyLister(mapIndexer)
		c.mapbindingBetaLister = admissionregistrationv1beta1listers.NewMutatingAdmissionPolicyBindingLister(mapBindingIndexer)
	case admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1:
		c.mapAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyLister(mapIndexer)
		c.mapbindingAlphaLister = admissionregistrationv1alpha1listers.NewMutatingAdmissionPolicyBindingLister(mapBindingIndexer)
	default:
		t.Fatalf("unknown MutatingAdmissionPolicy version %q", version)
	}
	return c, kubeClient, kyvernoClient
}

func getMpol(t *testing.T, client *versionedfake.Clientset, name string) *policiesv1beta1.MutatingPolicy {
	t.Helper()
	mpol, err := client.PoliciesV1beta1().MutatingPolicies().Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	return mpol
}

// TestHandleMAPGeneration_GeneratesForEachVersion covers the success path of every
// MutatingAdmissionPolicy version, since the status write moved out of the per-version handlers.
func TestHandleMAPGeneration_GeneratesForEachVersion(t *testing.T) {
	t.Parallel()
	for _, vc := range mapVersionCases() {
		t.Run(string(vc.version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mpol := generatedMpol()
			mpol.Status.Generated = false
			c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol)

			require.NoError(t, c.handleMAPGeneration(ctx, mpol))

			assert.NoError(t, vc.getMAP(ctx, kubeClient), "a MutatingAdmissionPolicy is expected")
			assert.NoError(t, vc.getMAPBinding(ctx, kubeClient), "a MutatingAdmissionPolicyBinding is expected")
			updated := getMpol(t, kyvernoClient, mpol.Name)
			assert.True(t, updated.Status.Generated)
			assert.Empty(t, updated.Status.GetConditionStatus().Message)
		})
	}
}

// TestHandleMAPGeneration_FailureHandsAdmissionBackToKyverno is the MutatingAdmissionPolicy
// counterpart of TestHandleVAPGeneration_FailureHandsAdmissionBackToKyverno, for every version.
// Unlike a stale VAP, a stale MAP is removed: it would keep mutating on top of Kyverno.
func TestHandleMAPGeneration_FailureHandsAdmissionBackToKyverno(t *testing.T) {
	t.Parallel()
	failures := []struct {
		name     string
		stale    bool
		resource string
		verb     string
		err      error
		// mapLeft is set when the MutatingAdmissionPolicy was created in this run: the informer
		// hasn't seen it yet, so only a retry removes it. It can't mutate anything without a binding.
		mapLeft bool
	}{{
		name:     "update of the existing MutatingAdmissionPolicy is rejected",
		stale:    true,
		resource: "mutatingadmissionpolicies",
		verb:     "update",
		err:      rejectedExpressionError(mapGroupKind, testMAPName),
	}, {
		name:     "MutatingAdmissionPolicy deleted out of band and re-creating it fails",
		resource: "mutatingadmissionpolicies",
		verb:     "create",
		err:      unavailableError(),
	}, {
		name:     "binding update fails after the MutatingAdmissionPolicy was updated",
		stale:    true,
		resource: "mutatingadmissionpolicybindings",
		verb:     "update",
		err:      unavailableError(),
	}, {
		name:     "binding creation fails after the MutatingAdmissionPolicy was created",
		resource: "mutatingadmissionpolicybindings",
		verb:     "create",
		err:      unavailableError(),
		mapLeft:  true,
	}}
	for _, vc := range mapVersionCases() {
		for _, tt := range failures {
			t.Run(string(vc.version)+"/"+tt.name, func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				mpol := generatedMpol()
				var existing []runtime.Object
				if tt.stale {
					existing = vc.stale()
				}
				c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, existing...)
				kubeClient.PrependReactor(tt.verb, tt.resource, func(clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, tt.err
				})

				require.Error(t, c.handleMAPGeneration(ctx, mpol), "the generation error must be returned so the workqueue retries")

				updated := getMpol(t, kyvernoClient, mpol.Name)
				assert.False(t, updated.Status.Generated, "Kyverno must evaluate the policy again")
				assert.Contains(t, updated.Status.GetConditionStatus().Message, "failed to generate MutatingAdmissionPolicy")
				mapErr := vc.getMAP(ctx, kubeClient)
				assert.Equal(t, tt.mapLeft, mapErr == nil, "MutatingAdmissionPolicy left until a retry, err=%v", mapErr)
				bindingErr := vc.getMAPBinding(ctx, kubeClient)
				assert.True(t, apierrors.IsNotFound(bindingErr), "the stale binding must be removed: got err=%v", bindingErr)
			})
		}
	}
}

// TestHandleMAPGeneration_BindingCreateFailureCleansUpAndRecovers follows a MutatingAdmissionPolicy
// whose binding can't be created. The first run leaves the new MAP, which the informer hasn't seen
// yet; once it has, the retry removes it while the binding still fails, and generation recovers
// once the binding can be created again.
func TestHandleMAPGeneration_BindingCreateFailureCleansUpAndRecovers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vc := mapVersionCases()[0]
	mpol := generatedMpol()
	c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol)
	mapIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	c.mapV1Lister = admissionregistrationv1listers.NewMutatingAdmissionPolicyLister(mapIndexer)
	bindingFails := true
	kubeClient.PrependReactor("create", "mutatingadmissionpolicybindings", func(clienttesting.Action) (bool, runtime.Object, error) {
		if bindingFails {
			return true, nil, unavailableError()
		}
		return false, nil, nil
	})

	require.Error(t, c.handleMAPGeneration(ctx, mpol))
	created, err := kubeClient.AdmissionregistrationV1().MutatingAdmissionPolicies().Get(ctx, testMAPName, metav1.GetOptions{})
	require.NoError(t, err, "the MAP created in this run is only removed on the retry")
	assert.False(t, getMpol(t, kyvernoClient, mpol.Name).Status.Generated)

	created.ResourceVersion = "1" // the informer now has the MAP
	require.NoError(t, mapIndexer.Add(created))
	require.Error(t, c.handleMAPGeneration(ctx, mpol))
	assert.True(t, apierrors.IsNotFound(vc.getMAP(ctx, kubeClient)), "the retry must remove the MAP while the binding still fails")
	assert.True(t, apierrors.IsNotFound(vc.getMAPBinding(ctx, kubeClient)))

	require.NoError(t, mapIndexer.Delete(created))
	bindingFails = false
	require.NoError(t, c.handleMAPGeneration(ctx, mpol))
	assert.NoError(t, vc.getMAP(ctx, kubeClient))
	assert.NoError(t, vc.getMAPBinding(ctx, kubeClient))
	assert.True(t, getMpol(t, kyvernoClient, mpol.Name).Status.Generated)
}

// TestHandleMAPGeneration_FallbackKeepsMAPWhenStatusWriteFails checks the order of the MAP fallback:
// the stale MutatingAdmissionPolicy is only removed once Kyverno has taken the policy back. If the
// status write fails, the MAP stays, so the policy is never left with nothing enforcing it.
func TestHandleMAPGeneration_FallbackKeepsMAPWhenStatusWriteFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vc := mapVersionCases()[0]
	mpol := generatedMpol()
	c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)
	kubeClient.PrependReactor("update", "mutatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, rejectedExpressionError(mapGroupKind, testMAPName)
	})
	kyvernoClient.PrependReactor("update", "mutatingpolicies", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(action.GetResource().GroupResource(), mpol.Name, nil)
	})

	err := c.handleMAPGeneration(ctx, mpol)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "the status conflict must reach the workqueue: %v", err)
	assert.NoError(t, vc.getMAP(ctx, kubeClient), "the MutatingAdmissionPolicy must stay until Kyverno has taken the policy back")
}

// TestHandleMAPGeneration_FailedStaleMAPDeleteIsRetried covers a stale MutatingAdmissionPolicy that
// cannot be deleted right away. Kyverno has already taken the policy back, and the error is returned
// so the workqueue retries the delete.
func TestHandleMAPGeneration_FailedStaleMAPDeleteIsRetried(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vc := mapVersionCases()[0]
	mpol := generatedMpol()
	c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)
	kubeClient.PrependReactor("update", "mutatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, rejectedExpressionError(mapGroupKind, testMAPName)
	})
	kubeClient.PrependReactor("delete", "mutatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, unavailableError()
	})

	err := c.handleMAPGeneration(ctx, mpol)
	require.Error(t, err)
	assert.True(t, apierrors.IsServiceUnavailable(err), "the failed delete must reach the workqueue: %v", err)
	updated := getMpol(t, kyvernoClient, mpol.Name)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "failed to generate MutatingAdmissionPolicy")
}

// TestReconcile_MAPOptOutRemovesGeneratedPolicy covers a MutatingPolicy that opted in to MAP
// generation and then opts out while the generateMutatingAdmissionPolicy flag is off, its default.
// The policy no longer asks for a MAP, but the one generated earlier still has to be removed and
// admission handed back to Kyverno.
func TestReconcile_MAPOptOutRemovesGeneratedPolicy(t *testing.T) {
	t.Parallel()
	for _, vc := range mapVersionCases() {
		t.Run(string(vc.version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mpol := generatedMpol()
			mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = ptr.To(false)
			c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))

			mapErr := vc.getMAP(ctx, kubeClient)
			assert.True(t, apierrors.IsNotFound(mapErr), "the MutatingAdmissionPolicy must be removed: got err=%v", mapErr)
			bindingErr := vc.getMAPBinding(ctx, kubeClient)
			assert.True(t, apierrors.IsNotFound(bindingErr), "the binding must be removed: got err=%v", bindingErr)
			updated := getMpol(t, kyvernoClient, mpol.Name)
			assert.False(t, updated.Status.Generated)
			assert.Equal(t, "skip generating MutatingAdmissionPolicy: not enabled.", updated.Status.GetConditionStatus().Message)
		})
	}
}

// TestReconcile_MAPOptOutRetriesFailedDelete covers the retry of a failed delete after such an
// opt-out. The status is cleared before the delete, so the retry reads generated=false and still
// has to find and remove the MutatingAdmissionPolicy, or it would keep applying the old mutations
// on top of Kyverno's.
func TestReconcile_MAPOptOutRetriesFailedDelete(t *testing.T) {
	t.Parallel()
	for _, vc := range mapVersionCases() {
		t.Run(string(vc.version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mpol := generatedMpol()
			mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = ptr.To(false)
			c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)
			failFirstDelete(kubeClient, "mutatingadmissionpolicies")

			require.Error(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))
			retried := getMpol(t, kyvernoClient, mpol.Name)
			require.False(t, retried.Status.Generated)
			c.mpolLister = mpolListerWith(t, retried)
			require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))

			mapErr := vc.getMAP(ctx, kubeClient)
			assert.True(t, apierrors.IsNotFound(mapErr), "the MutatingAdmissionPolicy must be removed: got err=%v", mapErr)
			bindingErr := vc.getMAPBinding(ctx, kubeClient)
			assert.True(t, apierrors.IsNotFound(bindingErr), "the binding must be removed: got err=%v", bindingErr)
		})
	}
}

// TestReconcile_MAPFlagOffLeavesHandMadePolicy covers a policy that never had a
// MutatingAdmissionPolicy generated, with the flag off. Objects that only share the generated names
// were not written by Kyverno and are left alone.
func TestReconcile_MAPFlagOffLeavesHandMadePolicy(t *testing.T) {
	t.Parallel()
	for _, vc := range mapVersionCases() {
		t.Run(string(vc.version), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			mpol := generatedMpol()
			mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = ptr.To(false)
			mpol.Status.Generated = false
			c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, withOwners(nil, vc.stale()...)...)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "MutatingPolicy/"+mpol.Name, "", mpol.Name))

			assert.Empty(t, kubeClient.Actions(), "objects Kyverno did not generate must be left alone")
			assert.Empty(t, kyvernoClient.Actions(), "the status must not be written while the flag is off")
		})
	}
}

// denyingAuthChecker denies every verb on one resource and allows everything else.
type denyingAuthChecker struct{ resource string }

func (d denyingAuthChecker) Check(_ context.Context, _, _, resource, _, _, _, _ string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: resource != d.resource}, nil
}

// TestHandleMAPGeneration_LostPermissionRemovesStaleMAP covers a controller that loses its
// permissions on MutatingAdmissionPolicies or their bindings after generating one. Admission goes
// back to Kyverno, so the MutatingAdmissionPolicy has to go as well, or it would keep applying the
// old mutations on top of Kyverno's. The checker only reports what Kyverno may do; the fake API
// server still accepts the delete, as it would when only create or update were taken away.
func TestHandleMAPGeneration_LostPermissionRemovesStaleMAP(t *testing.T) {
	t.Parallel()
	for _, resource := range []string{"mutatingadmissionpolicies", "mutatingadmissionpolicybindings"} {
		for _, vc := range mapVersionCases() {
			t.Run(resource+"/"+string(vc.version), func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				mpol := generatedMpol()
				c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)
				c.checker = denyingAuthChecker{resource: resource}

				require.ErrorContains(t, c.handleMAPGeneration(ctx, mpol), "insufficient permissions to generate")

				mapErr := vc.getMAP(ctx, kubeClient)
				assert.True(t, apierrors.IsNotFound(mapErr), "the MutatingAdmissionPolicy must be removed: got err=%v", mapErr)
				bindingErr := vc.getMAPBinding(ctx, kubeClient)
				assert.True(t, apierrors.IsNotFound(bindingErr), "the binding must be removed: got err=%v", bindingErr)
				updated := getMpol(t, kyvernoClient, mpol.Name)
				assert.False(t, updated.Status.Generated)
				assert.Contains(t, updated.Status.GetConditionStatus().Message, "insufficient permissions to generate")
			})
		}
	}
}

// TestHandleMAPGeneration_LostPermissionReturnsFailedDelete covers a controller that may no longer
// delete the MutatingAdmissionPolicy either. Admission is still handed back to Kyverno, and the
// error reaches the workqueue so the delete is retried instead of silently dropped.
func TestHandleMAPGeneration_LostPermissionReturnsFailedDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vc := mapVersionCases()[0]
	mpol := generatedMpol()
	c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, vc.stale()...)
	c.checker = denyingAuthChecker{resource: "mutatingadmissionpolicies"}
	kubeClient.PrependReactor("delete", "mutatingadmissionpolicies", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: mapGroupKind.Group, Resource: "mutatingadmissionpolicies"}, testMAPName, nil)
	})

	err := c.handleMAPGeneration(ctx, mpol)

	assert.True(t, apierrors.IsForbidden(err), "the failed delete must reach the workqueue: %v", err)
	updated := getMpol(t, kyvernoClient, mpol.Name)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "insufficient permissions to generate MutatingAdmissionPolicies")
}

// TestHandleMAPGeneration_LeavesObjectsKyvernoDidNotGenerate is the same for each way the
// MutatingAdmissionPolicy is removed while the policy stays: an opt-out, and lost permissions.
func TestHandleMAPGeneration_LeavesObjectsKyvernoDidNotGenerate(t *testing.T) {
	t.Parallel()
	leftOver := generatedBy("MutatingPolicy", testMpolName, "uid-of-a-deleted-policy")
	for _, path := range []struct {
		name   string
		optOut bool
		deny   string
	}{{name: "opt-out", optOut: true}, {name: "lost permission", deny: "mutatingadmissionpolicies"}} {
		for _, vc := range mapVersionCases() {
			for _, tc := range []struct {
				name                 string
				existing             func() []runtime.Object
				keepMAP, keepBinding bool
			}{
				{"created by hand", func() []runtime.Object { return withOwners(nil, vc.stale()...) }, true, true},
				{"left over from a deleted policy", func() []runtime.Object { return withOwners(leftOver, vc.stale()...) }, true, true},
				{"only the policy generated", func() []runtime.Object { objs := vc.stale(); withOwners(nil, objs[1]); return objs }, false, true},
			} {
				t.Run(path.name+"/"+string(vc.version)+"/"+tc.name, func(t *testing.T) {
					t.Parallel()
					ctx := context.Background()
					mpol := generatedMpol()
					if path.optOut {
						mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = ptr.To(false)
					}
					c, kubeClient, kyvernoClient := newVersionedMAPTestController(t, vc.version, mpol, tc.existing()...)
					if path.deny != "" {
						c.checker = denyingAuthChecker{resource: path.deny}
					}

					err := c.handleMAPGeneration(ctx, mpol)
					assert.Equal(t, path.deny != "", err != nil, "only a lost permission keeps a retry queued: %v", err)

					mapErr := vc.getMAP(ctx, kubeClient)
					assert.Equal(t, tc.keepMAP, mapErr == nil, "MutatingAdmissionPolicy kept, err=%v", mapErr)
					bindingErr := vc.getMAPBinding(ctx, kubeClient)
					assert.Equal(t, tc.keepBinding, bindingErr == nil, "binding kept, err=%v", bindingErr)
					assert.False(t, getMpol(t, kyvernoClient, mpol.Name).Status.Generated)
				})
			}
		}
	}
}

// TestHandleMAPGeneration_NoMAPAPIClearsGenerated covers a policy still marked generated on a
// cluster that no longer serves any MutatingAdmissionPolicy API, so no MAP can be enforcing it.
func TestHandleMAPGeneration_NoMAPAPIClearsGenerated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mpol := generatedMpol()
	mpol.Spec.AutogenConfiguration.MutatingAdmissionPolicy.Enabled = ptr.To(false)
	kyvernoClient := versionedfake.NewSimpleClientset(mpol)
	c := &controller{kyvernoClient: kyvernoClient}

	require.NoError(t, c.handleMAPGeneration(ctx, mpol))

	assert.False(t, getMpol(t, kyvernoClient, mpol.Name).Status.Generated)
}

// TestHandleVAPGeneration_NoVAPAPIHandsAdmissionBackToKyverno covers a cluster that serves no
// ValidatingAdmissionPolicy API, where the controller only runs for MutatingAdmissionPolicies.
// A policy still marked generated goes back to Kyverno.
func TestHandleVAPGeneration_NoVAPAPIHandsAdmissionBackToKyverno(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vpol := generatedVpol()
	c, _, kyvernoClient := newVAPTestController(t, vpol)
	c.vapLister = nil
	c.vapbindingLister = nil

	require.NoError(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)))

	updated := getVpol(t, kyvernoClient, vpol.Name)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "no ValidatingAdmissionPolicy API")
}

// TestHandleVAPGeneration_FailedBindingDeleteHandsAdmissionBackToKyverno covers turning VAP generation off
// when the ValidatingAdmissionPolicy is deleted but its binding is not. With the status written last, this
// used to leave the policy marked generated with no ValidatingAdmissionPolicy left to enforce it.
func TestHandleVAPGeneration_FailedBindingDeleteHandsAdmissionBackToKyverno(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	vpol := generatedVpol()
	vpol.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
	c, kubeClient, kyvernoClient := newVAPTestController(t, vpol, staleVAP(), staleVAPBinding())
	kubeClient.PrependReactor("delete", "validatingadmissionpolicybindings", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, unavailableError()
	})

	require.Error(t, c.handleVAPGeneration(ctx, engineapi.NewValidatingPolicy(vpol)), "the failed delete must be retried")

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, testVAPName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr), "the ValidatingAdmissionPolicy is deleted before the binding: got err=%v", vapErr)
	updated := getVpol(t, kyvernoClient, vpol.Name)
	assert.False(t, updated.Status.Generated, "with the ValidatingAdmissionPolicy gone, only Kyverno can enforce the policy")
}

// TestHandleMAPGenerationWithVersion_UnsupportedVersion covers the default branch of the version switch.
// preferredMAPVersion never returns an unknown version today, but if it did, a policy marked generated
// must not be left to a MutatingAdmissionPolicy that was never written.
func TestHandleMAPGenerationWithVersion_UnsupportedVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mpol := generatedMpol()
	c, _, kyvernoClient := newVersionedMAPTestController(t, admissionpolicy.MutatingAdmissionPolicyVersionV1, mpol)

	err := c.handleMAPGenerationWithVersion(ctx, mpol, admissionpolicy.MutatingAdmissionPolicyVersion("v2"))
	require.ErrorContains(t, err, "unsupported MutatingAdmissionPolicy version")

	updated := getMpol(t, kyvernoClient, mpol.Name)
	assert.False(t, updated.Status.Generated)
	assert.Contains(t, updated.Status.GetConditionStatus().Message, "failed to generate MutatingAdmissionPolicy")
}
