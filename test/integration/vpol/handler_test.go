//go:build integration

package vpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testEnv  *framework.TestEnv
	engine   vpolengine.Engine
	provider vpolengine.Provider
)

func TestMain(m *testing.M) {
	var err error
	testEnv, err = framework.NewTestEnv(
		"../../../config/crds/policies.kyverno.io",
	)
	if err != nil {
		panic(err)
	}

	engine, provider, err = framework.NewVpolEngine(testEnv.Mgr)
	if err != nil {
		testEnv.Stop()
		panic(err)
	}

	if err := testEnv.Start(); err != nil {
		testEnv.Stop()
		panic(err)
	}

	code := m.Run()
	testEnv.Stop()
	os.Exit(code)
}

// waitForPolicyReady waits until the provider has compiled at least count policies.
func waitForPolicyReady(t *testing.T, count int) {
	t.Helper()
	ctx := context.Background()
	require.Eventually(t, func() bool {
		policies, err := provider.Fetch(ctx)
		return err == nil && len(policies) >= count
	}, 5*time.Second, 100*time.Millisecond, "policies not reconciled in time")
}

// waitForPolicyGone waits until the provider has no compiled policies.
func waitForPolicyGone(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	require.Eventually(t, func() bool {
		policies, err := provider.Fetch(ctx)
		return err == nil && len(policies) == 0
	}, 5*time.Second, 100*time.Millisecond, "policies not cleaned up in time")
}

// createPolicyWithCleanup creates a ValidatingPolicy and registers cleanup.
func createPolicyWithCleanup(t *testing.T, policy *policiesv1beta1.ValidatingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
}

func TestValidate_DenyPolicy_BlocksNonCompliantResource(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-production-pods"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(object.metadata.labels) || !('env' in object.metadata.labels) || object.metadata.labels.env != 'production'",
				Message:    "production pods are not allowed in this cluster",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("prod-app", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "prod-app", "namespace": "default", "labels": {"env": "production"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "production pod should be denied")
}

func TestValidate_DenyPolicy_AllowsCompliantResource(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-production-pods-allow"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "!has(object.metadata.labels) || !('env' in object.metadata.labels) || object.metadata.labels.env != 'production'",
				Message:    "production pods are not allowed in this cluster",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("staging-app", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "staging-app", "namespace": "default", "labels": {"env": "staging"}},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "staging pod should be allowed")
}

func TestValidate_WarnPolicy_AllowsButWarns(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "warn-no-resource-limits"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.containers.all(c, has(c.resources) && has(c.resources.limits))",
				Message:    "all containers must have resource limits",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("no-limits-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "no-limits-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "warn-only policy should allow the resource")
	assert.NotEmpty(t, resp.Warnings, "warn policy should produce warnings for non-compliant resource")
}

func TestValidate_MatchCondition_SkipsNonMatchingResource(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-only-in-production-ns"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "only-production-namespace",
				Expression: "object.metadata.namespace == 'production'",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'approved' in object.metadata.labels",
				Message:    "pods in production namespace must have approved label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("my-app", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "my-app", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "pod in non-production namespace should be allowed (match condition not met)")
}

func TestValidate_EventsGenerated_OnDeny(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-latest-tag"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))",
				Message:    "containers must not use the latest tag",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("bad-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "bad-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx:latest"}]}
	}`)), "", time.Now())

	// Wait for the async audit goroutine to generate events
	time.Sleep(200 * time.Millisecond)

	assert.False(t, resp.Allowed, "pod with latest tag should be denied")
	assert.NotEmpty(t, eventGen.Events, "deny should generate events")
}

func TestValidate_CELVariables_UsedInValidation(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-app-label"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{{
				Name:       "appLabel",
				Expression: "has(object.metadata.labels) && 'app' in object.metadata.labels ? object.metadata.labels.app : ''",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "variables.appLabel != ''",
				Message:    "pods must have an app label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("no-label-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "no-label-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "pod without app label should be denied when using CEL variable")
}

func TestValidate_AuditAction_AllowsNonCompliantButFiresEvents(t *testing.T) {
	// Security team deploys a policy in audit mode to observe which pods would
	// violate the rule before switching to enforce. Non-compliant pods should
	// pass through, but the violation is recorded via events for review.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "audit-require-team-label"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must have a team label for cost tracking",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("unlabeled-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "unlabeled-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	// Audit mode: resource passes through even though it violates the policy.
	assert.True(t, resp.Allowed, "audit policy should not block the resource")
	assert.Empty(t, resp.Warnings, "audit policy should not produce warnings")

	// Wait for the async audit goroutine to record the violation.
	time.Sleep(200 * time.Millisecond)
	assert.NotEmpty(t, eventGen.Events, "audit policy should still generate events for observability")
}

func TestValidate_MultiplePolicies_DenyAndWarnCombined(t *testing.T) {
	// Real clusters have multiple policies from different teams. A security
	// team's deny policy blocks latest tags while a platform team's warn policy
	// flags missing resource limits. Both should fire on the same pod.
	denyPolicy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-latest-image"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))",
				Message:    "latest image tag is not allowed",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	warnPolicy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "warn-missing-limits"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.spec.containers.all(c, has(c.resources) && has(c.resources.limits))",
				Message:    "containers should have resource limits",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Warn},
		},
	}

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, denyPolicy))
	require.NoError(t, testEnv.Client.Create(ctx, warnPolicy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), denyPolicy)
		testEnv.Client.Delete(context.Background(), warnPolicy)
		waitForPolicyGone(t)
	})
	waitForPolicyReady(t, 2)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	policyCtx := framework.ContextWithPolicies(context.Background(), "deny-latest-image", "warn-missing-limits")
	resp := h.ValidateClustered(policyCtx, logr.Discard(), framework.PodAdmissionRequest("bad-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "bad-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx:latest"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "deny policy should block the resource")
	assert.NotEmpty(t, resp.Warnings, "warn policy should still produce warnings even when denied")
}

func TestValidate_MultipleValidations_PartialFailureDenies(t *testing.T) {
	// Admin writes a policy with two compliance checks. A pod passes the image
	// tag check but fails the label check. The deny should fire with a message
	// about the specific failing validation.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "multi-check-compliance"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))",
					Message:    "containers must not use the latest tag",
				},
				{
					Expression: "has(object.metadata.labels) && 'cost-center' in object.metadata.labels",
					Message:    "pods must have a cost-center label",
				},
			},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	// Pod uses nginx:1.25 (passes image check) but has no cost-center label (fails label check)
	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("partial-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "partial-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx:1.25"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "pod missing cost-center label should be denied")
	require.NotNil(t, resp.Result, "deny response should include result details")
	assert.Contains(t, resp.Result.Message, "cost-center", "error should mention the failing validation")
}
