//go:build integration

package vpol_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

// These tests change or delete a ValidatingPolicy while the handler is serving and check that
// the next admission sees the change. Every other test creates a policy once and leaves it, so
// they never exercise the reconciler recompiling or evicting an already compiled policy.

var lifecyclePod = []byte(`{
	"apiVersion": "v1", "kind": "Pod",
	"metadata": {"name": "unlabeled", "namespace": "default"},
	"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
}`)

func requireTeamLabelPolicy(name string, action admissionregistrationv1.ValidationAction) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must have a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{action},
		},
	}
}

// updatePolicy re-reads the policy and applies mutate, retrying on a conflict with the reconciler
// writing status.
func updatePolicy(t *testing.T, name string, mutate func(*policiesv1beta1.ValidatingPolicy)) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var current policiesv1beta1.ValidatingPolicy
		if err := testEnv.Client.Get(context.Background(), types.NamespacedName{Name: name}, &current); err != nil {
			return err
		}
		mutate(&current)
		return testEnv.Client.Update(context.Background(), &current)
	})
	require.NoError(t, err)
}

func TestValidate_PolicyUpdate_SwitchingDenyToAuditTakesEffect(t *testing.T) {
	// A rollout goes wrong and the platform team flips an enforcing policy back to Audit to
	// unblock deploys. Admission must start allowing without restarting anything, and must
	// keep allowing: a stale compiled copy would keep denying.
	const policyName = "flip-deny-to-audit"
	createPolicyWithCleanup(t, requireTeamLabelPolicy(policyName, admissionregistrationv1.Deny))
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	admit := func() bool {
		return h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("unlabeled", "default", lifecyclePod), "", time.Now()).Allowed
	}

	require.False(t, admit(), "the policy must deny while it is in Deny mode")

	updatePolicy(t, policyName, func(p *policiesv1beta1.ValidatingPolicy) {
		p.Spec.ValidationAction = []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}
	})

	require.Eventually(t, admit, 5*time.Second, 100*time.Millisecond, "switching to Audit must stop the policy from denying")
	assert.True(t, admit(), "the recompiled Audit policy must keep admitting")
}

func TestValidate_PolicyUpdate_ChangedExpressionReplacesTheOldRule(t *testing.T) {
	// The policy is edited to check a different label. The new rule must apply and the old
	// rule must be gone, not evaluated alongside it.
	const policyName = "edit-required-label"
	createPolicyWithCleanup(t, requireTeamLabelPolicy(policyName, admissionregistrationv1.Deny))
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	teamOnly := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "team-only", "namespace": "default", "labels": {"team": "payments"}},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)
	ownerOnly := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "owner-only", "namespace": "default", "labels": {"owner": "payments"}},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)
	admit := func(name string, raw []byte) bool {
		return h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest(name, "default", raw), "", time.Now()).Allowed
	}

	require.True(t, admit("team-only", teamOnly), "the original rule must admit a pod with a team label")
	require.False(t, admit("owner-only", ownerOnly), "the original rule must deny a pod without a team label")

	updatePolicy(t, policyName, func(p *policiesv1beta1.ValidatingPolicy) {
		p.Spec.Validations = []admissionregistrationv1.Validation{{
			Expression: "has(object.metadata.labels) && 'owner' in object.metadata.labels",
			Message:    "pods must have an owner label",
		}}
	})

	require.Eventually(t, func() bool {
		return admit("owner-only", ownerOnly)
	}, 5*time.Second, 100*time.Millisecond, "the edited rule must admit a pod with an owner label")

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("team-only", "default", teamOnly), "", time.Now())
	assert.False(t, resp.Allowed, "the edited rule must now deny a pod with only a team label")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "pods must have an owner label")
	assert.NotContains(t, resp.Result.Message, "pods must have a team label", "the old rule must not be evaluated any more")
}

func TestValidate_PolicyDelete_StopsEnforcement(t *testing.T) {
	// Deleting a policy must remove it from admission. The handler is called without a
	// policy-name filter, the way the catch-all webhook route calls it, so a policy left in the
	// provider after deletion would still deny.
	const policyName = "delete-me-require-team"
	policy := requireTeamLabelPolicy(policyName, admissionregistrationv1.Deny)
	require.NoError(t, testEnv.Client.Create(context.Background(), policy))
	t.Cleanup(func() {
		_ = testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	admit := func() bool {
		return h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("unlabeled", "default", lifecyclePod), "", time.Now()).Allowed
	}

	require.False(t, admit(), "the policy must deny before it is deleted")

	require.NoError(t, testEnv.Client.Delete(context.Background(), policy))
	waitForPolicyGone(t)

	assert.True(t, admit(), "a deleted policy must no longer deny")
}
