//go:build integration

package vpol_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
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

	engine, provider, err = framework.NewVpolEngineWithExceptions(testEnv.Mgr)
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

// --- Namespaced policy tests ---

// createNamespacedPolicyWithCleanup creates a NamespacedValidatingPolicy and registers cleanup.
func createNamespacedPolicyWithCleanup(t *testing.T, policy *policiesv1beta1.NamespacedValidatingPolicy) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
}

func TestValidateNamespaced_PolicyOnlyAppliesToItsNamespace(t *testing.T) {
	// Multi-tenant cluster: team-a deploys a NamespacedValidatingPolicy that requires
	// an "owner" label on all pods. team-b's pods in a different namespace should
	// not be affected — the policy is scoped to team-a only.
	framework.CreateNamespace(t, testEnv.KubeClient, "team-a")
	framework.CreateNamespace(t, testEnv.KubeClient, "team-b")

	policy := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "require-owner-label",
			Namespace: "team-a",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'owner' in object.metadata.labels",
				Message:    "pods must have an owner label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createNamespacedPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	// Pod in team-a (same namespace as policy) — should be denied.
	ctx := framework.ContextWithPolicies(context.Background(), "require-owner-label")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app-pod", "team-a", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "app-pod", "namespace": "team-a"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "pod in team-a should be denied by namespaced policy")

	// Pod in team-b (different namespace) — policy should not apply.
	resp = h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("app-pod", "team-b", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "app-pod", "namespace": "team-b"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "pod in team-b should not be affected by team-a's namespaced policy")
}

func TestValidate_PolicyExceptionSkipsValidation(t *testing.T) {
	// Break-glass scenario: a DB migration pod needs to bypass the label-enforcement
	// policy temporarily. The platform team creates a PolicyException so the migration
	// can proceed without being blocked.
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-team-label-ex"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "pods must have a team label",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	waitForPolicyReady(t, 1)

	// Verify the policy actually denies before the exception is created.
	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)
	podJSON := []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "db-migration", "namespace": "default"},
		"spec": {"containers": [{"name": "migrate", "image": "postgres:16"}]}
	}`)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("db-migration", "default", podJSON), "", time.Now())
	require.False(t, resp.Allowed, "policy should deny before exception is created")

	// Now create the exception — the reconciler will re-compile the policy with exception data.
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-db-migration",
			Namespace: "default",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "require-team-label-ex",
				Kind: "ValidatingPolicy",
			}},
		},
	}

	require.NoError(t, testEnv.Client.Create(ctx, exception))

	// Manage cleanup explicitly: delete exception first and wait for the
	// re-reconciliation it triggers to settle, then delete the policy.
	// This avoids the race where exception-delete re-queues the policy
	// right as we're trying to delete it.
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), exception)
		// Wait for the exception-delete-triggered re-reconciliation to complete
		// before deleting the policy.
		time.Sleep(500 * time.Millisecond)
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})

	// Wait for the exception to take effect by polling the handler.
	// The exception watch re-queues the policy, and the reconciler re-compiles
	// it with exception data from the same manager cache — no dual-cache race.
	require.Eventually(t, func() bool {
		resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("db-migration", "default", podJSON), "", time.Now())
		return resp.Allowed
	}, 5*time.Second, 200*time.Millisecond, "exception should make the policy skip validation")
}

func TestValidate_FailurePolicy_FailBlocksOnMatchConditionError(t *testing.T) {
	// An admin writes a match condition that references a field that doesn't exist
	// on all resource types. With failurePolicy=Fail (the default), a CEL eval
	// error in the match condition should block admission rather than silently
	// letting non-matching resources through.
	fail := admissionregistrationv1.Fail
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "fail-on-match-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &fail,
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "broken-condition",
				Expression: "object.spec.nonExistentField == 'x'",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "true",
				Message:    "always passes",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "test-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.False(t, resp.Allowed, "failurePolicy=Fail should block when match condition errors")
}

func TestValidate_FailurePolicy_IgnoreSkipsOnMatchConditionError(t *testing.T) {
	// Same broken match condition, but failurePolicy=Ignore. The policy should
	// be skipped entirely — the pod goes through without being validated.
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-on-match-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "broken-condition",
				Expression: "object.spec.nonExistentField == 'x'",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "false",
				Message:    "always fails if evaluated",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "test-pod", "namespace": "default"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "failurePolicy=Ignore should skip policy when match condition errors")
}
