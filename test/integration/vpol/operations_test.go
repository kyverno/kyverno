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
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These tests cover the parts of the admission request a ValidatingPolicy can read beyond
// `object` on CREATE: oldObject on UPDATE and DELETE, request.userInfo, and a messageExpression
// rendered from the admitted object. Each one goes through the real handler, so they also check
// that the webhook route hands the engine the request the API server would send.

func podWithTeam(name, team string, extraLabels string) []byte {
	labels := `"team": "` + team + `"`
	if extraLabels != "" {
		labels += ", " + extraLabels
	}
	return []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "` + name + `", "namespace": "default", "labels": {` + labels + `}},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)
}

func TestValidate_UpdateOperation_OldObjectEnforcesImmutableLabel(t *testing.T) {
	// Cost allocation keys off the team label, so a platform team makes it immutable once a pod
	// exists. The rule compares oldObject with object, which only works if the handler passes
	// the stored object through on UPDATE.
	const policyName = "immutable-team-label"
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Update),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(oldObject.metadata.labels) || !('team' in oldObject.metadata.labels) || " +
					"(has(object.metadata.labels) && 'team' in object.metadata.labels && object.metadata.labels.team == oldObject.metadata.labels.team)",
				Message: "the team label is immutable",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	old := podWithTeam("billing-api", "payments", "")

	resp := h.ValidateClustered(ctx, logr.Discard(),
		framework.PodUpdateAdmissionRequest("billing-api", "default", old, podWithTeam("billing-api", "search", "")), "", time.Now())
	assert.False(t, resp.Allowed, "changing the team label must be denied")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "the team label is immutable")

	resp = h.ValidateClustered(ctx, logr.Discard(),
		framework.PodUpdateAdmissionRequest("billing-api", "default", old, podWithTeam("billing-api", "payments", `"version": "v2"`)), "", time.Now())
	assert.True(t, resp.Allowed, "an update that keeps the team label must be allowed: %v", resp.Result)
}

func TestValidate_UpdateOnlyPolicy_IgnoresCreate(t *testing.T) {
	// The policy above only matches UPDATE. A CREATE has no oldObject, so if the operation filter
	// in matchConstraints were ignored the rule would run against a null oldObject. A pod that
	// would fail the rule on UPDATE must therefore be admitted on CREATE.
	const policyName = "update-only-require-team"
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Update),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must keep a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	unlabeled := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "scratch", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("scratch", "default", unlabeled), "", time.Now())
	assert.True(t, resp.Allowed, "an UPDATE-only policy must not evaluate a CREATE: %v", resp.Result)

	resp = h.ValidateClustered(ctx, logr.Discard(),
		framework.PodUpdateAdmissionRequest("scratch", "default", podWithTeam("scratch", "payments", ""), unlabeled), "", time.Now())
	assert.False(t, resp.Allowed, "the same pod must be denied on UPDATE, proving the policy is live")
}

func TestValidate_DeleteOperation_OldObjectProtectsLabelledPods(t *testing.T) {
	// Deletion protection: pods labelled protected=true cannot be deleted. On DELETE the API
	// server sends the pod only as oldObject (object is null), so this exercises the handler
	// and engine taking the resource from oldObject.
	const policyName = "protect-pods-from-deletion"
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRulesWithOps(admissionregistrationv1.Delete),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!(has(oldObject.metadata.labels) && 'protected' in oldObject.metadata.labels && oldObject.metadata.labels.protected == 'true')",
				Message:    "protected pods cannot be deleted",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("etcd-0", "default", admissionv1.Delete,
		podWithTeam("etcd-0", "storage", `"protected": "true"`)), "", time.Now())
	assert.False(t, resp.Allowed, "deleting a protected pod must be denied")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "protected pods cannot be deleted")

	resp = h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestWithOp("worker-0", "default", admissionv1.Delete,
		podWithTeam("worker-0", "storage", "")), "", time.Now())
	assert.True(t, resp.Allowed, "deleting an unprotected pod must be allowed: %v", resp.Result)
}

func TestValidate_RequestUserInfo_OnlyCIMayCreateReleasePods(t *testing.T) {
	// Only the CI service account may create pods labelled release=true. The rule reads
	// request.userInfo, so the same pod must be admitted or denied depending on who sends it.
	const (
		policyName = "release-pods-from-ci-only"
		ciUser     = "system:serviceaccount:ci:deployer"
	)
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!(has(object.metadata.labels) && 'release' in object.metadata.labels) || request.userInfo.username == '" + ciUser + "'",
				Message:    "only the CI deployer may create release pods",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)
	releasePod := podWithTeam("api-v2", "payments", `"release": "true"`)

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestWithUsername("api-v2", "default", ciUser, releasePod), "", time.Now())
	assert.True(t, resp.Allowed, "the CI deployer must be allowed: %v", resp.Result)

	resp = h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequestWithUsername("api-v2", "default", "jane@example.com", releasePod), "", time.Now())
	assert.False(t, resp.Allowed, "any other user must be denied")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "only the CI deployer may create release pods")
}

func TestValidate_MessageExpression_RendersOffendingImageInDenial(t *testing.T) {
	// A denial that names the offending image tells the user what to fix. messageExpression is
	// evaluated against the admitted object, and its result must replace the static message.
	const policyName = "deny-latest-with-image-in-message"
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression:        "object.spec.containers.all(c, !c.image.endsWith(':latest'))",
				Message:           "static fallback message",
				MessageExpression: "'image ' + object.spec.containers.filter(c, c.image.endsWith(':latest'))[0].image + ' uses the latest tag'",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	createPolicyWithCleanup(t, policy)
	framework.WaitForVpols(t, provider, policyName)

	h := vpol.New(engine, testEnv.ContextProvider, nil, false, &framework.MockEventGen{})
	ctx := framework.ContextWithPolicies(context.Background(), policyName)

	resp := h.ValidateClustered(ctx, logr.Discard(), framework.PodAdmissionRequest("web", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "web", "namespace": "default"},
		"spec": {"containers": [
			{"name": "app", "image": "nginx:1.25"},
			{"name": "proxy", "image": "envoyproxy/envoy:latest"}
		]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "a pod with a latest-tagged image must be denied")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "image envoyproxy/envoy:latest uses the latest tag")
	assert.NotContains(t, resp.Result.Message, "static fallback message", "messageExpression must take precedence over message")
}
