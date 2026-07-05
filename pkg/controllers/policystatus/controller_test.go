package policystatus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	auth "github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

// emptyMatchResources is a convenience for constructing policy specs with no resource rules.
var emptyMatchResources = &admissionregistrationv1.MatchResources{
	ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
}

func TestRetryStatusUpdate_swallowsNotFound(t *testing.T) {
	t.Parallel()
	err := retryStatusUpdate(logr.Discard(), func() error {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "validatingpolicies"}, "missing")
	})
	assert.NoError(t, err)
}

func TestRetryStatusUpdate_conflictsThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	err := retryStatusUpdate(logr.Discard(), func() error {
		if calls.Add(1) == 1 {
			return apierrors.NewConflict(
				schema.GroupResource{Group: policiesv1beta1.SchemeGroupVersion.Group, Resource: "validatingpolicies"},
				"test",
				errors.New("the object has been modified"),
			)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "RetryOnConflict should rerun the function after Conflict")
}

func TestReconcile_ValidatingPolicy_retriesOnUpdateStatusConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
			},
		},
	}

	client := versionedfake.NewSimpleClientset(vpol)

	var statusUpdateCalls atomic.Int32
	client.PrependReactor("update", "validatingpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if statusUpdateCalls.Add(1) == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: policiesv1beta1.SchemeGroupVersion.Group, Resource: "validatingpolicies"},
				vpol.Name,
				errors.New("the object has been modified"),
			)
		}
		return false, nil, nil
	})

	dc := dclient.NewEmptyFakeClient()
	c := controller{
		dclient:          dc,
		client:           client,
		authChecker:      auth.NewSubjectChecker(dc.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), "", nil),
		polStateRecorder: webhook.NewStateRecorder(nil),
	}

	key := webhook.BuildRecorderKey(webhook.ValidatingPolicyType, vpol.Name, "")
	err := c.reconcile(ctx, logr.Discard(), key, "", "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, statusUpdateCalls.Load(), int32(2), "first UpdateStatus conflict should be retried")

	updated, err := client.PoliciesV1beta1().ValidatingPolicies().Get(ctx, vpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, updated.Status.ConditionStatus.Conditions)
}

// TestReconcile_DeletingPolicy_preservesLastExecutionTime verifies that the
// status controller (which owns ConditionStatus) does not clobber the
// LastExecutionTime field owned by the deleting controller.
func TestReconcile_DeletingPolicy_preservesLastExecutionTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	lastExec := metav1.NewTime(metav1.Now().Add(-time.Hour).Truncate(time.Second))
	dpol := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-dpol"},
		Spec:       policiesv1beta1.DeletingPolicySpec{MatchConstraints: emptyMatchResources},
		Status:     policiesv1beta1.DeletingPolicyStatus{LastExecutionTime: lastExec},
	}

	client := versionedfake.NewSimpleClientset(dpol)
	dc := dclient.NewEmptyFakeClient()
	c := controller{
		dclient:          dc,
		client:           client,
		authChecker:      auth.NewSubjectChecker(dc.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), "", nil),
		polStateRecorder: webhook.NewStateRecorder(nil),
	}

	key := webhook.BuildRecorderKey(webhook.DeletingPolicyType, dpol.Name, "")
	require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", ""))

	updated, err := client.PoliciesV1beta1().DeletingPolicies().Get(ctx, dpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotEmpty(t, updated.Status.ConditionStatus.Conditions, "ConditionStatus should be populated")
	assert.True(t, updated.Status.LastExecutionTime.Equal(&lastExec), "LastExecutionTime must be preserved, got %v want %v", updated.Status.LastExecutionTime, lastExec)
}

// TestReconcile_AllPolicyTypes verifies that reconcile() correctly routes each
// policy type to its update function, writes a non-empty status, and that
// BuildRecorderKey produces a routable key for every type. A missing case in
// the reconcile switch or in BuildRecorderKey would leave status empty.
func TestReconcile_AllPolicyTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		key           string
		objects       []runtime.Object
		setupRecorder func(webhook.StateRecorder)
		checkFunc     func(t *testing.T, client *versionedfake.Clientset)
	}{
		{
			name: "ValidatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.ValidatingPolicyType, "test-vpol", ""),
			objects: []runtime.Object{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
					Spec:       policiesv1beta1.ValidatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().ValidatingPolicies().Get(context.Background(), "test-vpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "ValidatingPolicy status should have conditions")
			},
		},
		{
			name: "NamespacedValidatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.NamespacedValidatingPolicyType, "test-nvpol", "test-ns"),
			objects: []runtime.Object{
				&policiesv1beta1.NamespacedValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-nvpol", Namespace: "test-ns"},
					Spec:       policiesv1beta1.ValidatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().NamespacedValidatingPolicies("test-ns").Get(context.Background(), "test-nvpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "NamespacedValidatingPolicy status should have conditions")
			},
		},
		{
			name: "ImageValidatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.ImageValidatingPolicyType, "test-ivpol", ""),
			objects: []runtime.Object{
				&policiesv1beta1.ImageValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ivpol"},
					Spec:       policiesv1beta1.ImageValidatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().ImageValidatingPolicies().Get(context.Background(), "test-ivpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "ImageValidatingPolicy status should have conditions")
			},
		},
		{
			name: "NamespacedImageValidatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.NamespacedImageValidatingPolicyType, "test-nivpol", "test-ns"),
			objects: []runtime.Object{
				&policiesv1beta1.NamespacedImageValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-nivpol", Namespace: "test-ns"},
					Spec:       policiesv1beta1.ImageValidatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().NamespacedImageValidatingPolicies("test-ns").Get(context.Background(), "test-nivpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "NamespacedImageValidatingPolicy status should have conditions")
			},
		},
		{
			name: "MutatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.MutatingPolicyType, "test-mpol", ""),
			objects: []runtime.Object{
				&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
					Spec:       policiesv1beta1.MutatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().MutatingPolicies().Get(context.Background(), "test-mpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "MutatingPolicy status should have conditions")
			},
		},
		{
			name: "NamespacedMutatingPolicy",
			key:  webhook.BuildRecorderKey(webhook.NamespacedMutatingPolicyType, "test-nmpol", "test-ns"),
			objects: []runtime.Object{
				&policiesv1beta1.NamespacedMutatingPolicy{
					TypeMeta:   metav1.TypeMeta{Kind: "NamespacedMutatingPolicy", APIVersion: "policies.kyverno.io/v1beta1"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "test-ns"},
					Spec:       policiesv1beta1.MutatingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			setupRecorder: func(r webhook.StateRecorder) {
				r.Record(webhook.BuildRecorderKey(webhook.NamespacedMutatingPolicyType, "test-nmpol", "test-ns"))
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().NamespacedMutatingPolicies("test-ns").Get(context.Background(), "test-nmpol", metav1.GetOptions{})
				require.NoError(t, err)
				var webhookCond *metav1.Condition
				for i := range pol.Status.ConditionStatus.Conditions {
					if pol.Status.ConditionStatus.Conditions[i].Type == string(policiesv1beta1.PolicyConditionTypeWebhookConfigured) {
						c := pol.Status.ConditionStatus.Conditions[i]
						webhookCond = &c
						break
					}
				}
				require.NotNil(t, webhookCond, "NamespacedMutatingPolicy WebhookConfigured condition should be set by the NMP branch in reconcileConditions")
				assert.Equal(t, metav1.ConditionTrue, webhookCond.Status, "WebhookConfigured should be True after recording the key")
			},
		},
		{
			name: "GeneratingPolicy",
			key:  webhook.BuildRecorderKey(webhook.GeneratingPolicyType, "test-gpol", ""),
			objects: []runtime.Object{
				&policiesv1beta1.GeneratingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-gpol"},
					Spec:       policiesv1beta1.GeneratingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().GeneratingPolicies().Get(context.Background(), "test-gpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "GeneratingPolicy status should have conditions")
			},
		},
		{
			name: "DeletingPolicy",
			key:  webhook.BuildRecorderKey(webhook.DeletingPolicyType, "test-dpol", ""),
			objects: []runtime.Object{
				&policiesv1beta1.DeletingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-dpol"},
					Spec:       policiesv1beta1.DeletingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().DeletingPolicies().Get(context.Background(), "test-dpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "DeletingPolicy status should have conditions")
				// Deleting policies have no webhook, so only the RBAC condition is expected.
				for i := range pol.Status.ConditionStatus.Conditions {
					assert.NotEqual(t, string(policiesv1beta1.PolicyConditionTypeWebhookConfigured), pol.Status.ConditionStatus.Conditions[i].Type,
						"DeletingPolicy should not report a WebhookConfigured condition")
				}
			},
		},
		{
			name: "NamespacedDeletingPolicy",
			key:  webhook.BuildRecorderKey(webhook.NamespacedDeletingPolicyType, "test-ndpol", "test-ns"),
			objects: []runtime.Object{
				&policiesv1beta1.NamespacedDeletingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-ndpol", Namespace: "test-ns"},
					Spec:       policiesv1beta1.DeletingPolicySpec{MatchConstraints: emptyMatchResources},
				},
			},
			checkFunc: func(t *testing.T, client *versionedfake.Clientset) {
				t.Helper()
				pol, err := client.PoliciesV1beta1().NamespacedDeletingPolicies("test-ns").Get(context.Background(), "test-ndpol", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotEmpty(t, pol.Status.ConditionStatus.Conditions, "NamespacedDeletingPolicy status should have conditions")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			client := versionedfake.NewSimpleClientset(tt.objects...)
			dc := dclient.NewEmptyFakeClient()
			recorder := webhook.NewStateRecorder(nil)
			if tt.setupRecorder != nil {
				tt.setupRecorder(recorder)
			}
			c := controller{
				dclient:          dc,
				client:           client,
				authChecker:      auth.NewSubjectChecker(dc.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), "", nil),
				polStateRecorder: recorder,
			}

			err := c.reconcile(ctx, logr.Discard(), tt.key, "", "")
			require.NoError(t, err)
			tt.checkFunc(t, client)
		})
	}
}
