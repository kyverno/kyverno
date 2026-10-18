//go:build integration

package vpol_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	vpol "github.com/kyverno/kyverno/pkg/webhooks/resource/vpol"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var unlabeledPodJSON = []byte(`{
	"apiVersion": "v1", "kind": "Pod",
	"metadata": {"name": "test-pod", "namespace": "default"},
	"spec": {"containers": [{"name": "app", "image": "nginx"}]}
}`)

func TestValidate_FailurePolicy_FailBlocksOnValidationRuntimeError(t *testing.T) {
	fail := admissionregistrationv1.Fail
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "fail-on-validation-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &fail,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.labels.team == 'platform'",
				Message:    "team label must be platform",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())

	assert.False(t, resp.Allowed, "failurePolicy=Fail should block when a validation errors at runtime")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "no such key", "the denial should come from the runtime error")
}

func TestValidate_FailurePolicy_IgnoreAllowsOnValidationRuntimeError(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-on-validation-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.labels.team == 'platform'",
				Message:    "team label must be platform",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())

	assert.True(t, resp.Allowed, "failurePolicy=Ignore should allow when a validation errors at runtime")
	assert.NotEmpty(t, eventGen.GetEvents(), "the ignored error should still be reported through events")
}

func TestValidate_FailurePolicy_IgnoreStillBlocksFailedValidation(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-still-denies-violation"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "team label is required",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())

	assert.False(t, resp.Allowed, "failurePolicy=Ignore must not hide a validation that evaluates to false")
}

func TestValidate_FailurePolicy_IgnoreAllowsOnVariableRuntimeError(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-on-variable-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Variables: []admissionregistrationv1.Variable{{
				Name:       "team",
				Expression: "object.metadata.labels.team",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "variables.team == 'platform'",
				Message:    "team label must be platform",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())

	assert.True(t, resp.Allowed, "failurePolicy=Ignore should allow when a variable errors at runtime")
	assert.NotEmpty(t, eventGen.GetEvents(), "the ignored error should still be reported through events")
}

func TestValidate_FailurePolicy_IgnoreStillBlocksWhenAuditAnnotationErrors(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-audit-annotation-error"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "team label is required",
			}},
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
				Key:             "team",
				ValueExpression: "string(object.metadata.labels.team)",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	resp := h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())

	assert.False(t, resp.Allowed, "an audit annotation error must not hide a failed validation")
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, "team label is required")
}

func TestValidate_FailurePolicy_IgnoreStillBlocksWhenExceptionControlsError(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-refused-exception"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
				Message:    "team label is required",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "broken-controls", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{Name: "ignore-refused-exception", Kind: "ValidatingPolicy"}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.labels.ticket != ''",
				Message:    "a ticket label is required to use this exception",
			}},
		},
	}

	ctx := context.Background()
	require.NoError(t, testEnv.Client.Create(ctx, policy))
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() {
		testEnv.Client.Delete(context.Background(), exception)
		time.Sleep(500 * time.Millisecond)
		testEnv.Client.Delete(context.Background(), policy)
		waitForPolicyGone(t)
	})
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	var resp handlers.AdmissionResponse
	require.Eventually(t, func() bool {
		resp = h.ValidateClustered(context.Background(), logr.Discard(), framework.PodAdmissionRequest("test-pod", "default", unlabeledPodJSON), "", time.Now())
		return resp.Result != nil && strings.Contains(resp.Result.Message, "compensating controls")
	}, 5*time.Second, 200*time.Millisecond, "the refused exception should be reported in the denial")
	assert.False(t, resp.Allowed, "a broken exception must not bypass a failed validation")
}

func TestValidateNamespaced_FailurePolicy_IgnoreAllowsOnValidationRuntimeError(t *testing.T) {
	framework.CreateNamespace(t, testEnv.KubeClient, "fp-ignore")
	ignore := admissionregistrationv1.Ignore
	policy := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "ignore-on-validation-error", Namespace: "fp-ignore"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy:    &ignore,
			MatchConstraints: framework.PodMatchRules(),
			Validations: []admissionregistrationv1.Validation{{
				Expression: "object.metadata.labels.team == 'platform'",
				Message:    "team label must be platform",
			}},
			ValidationAction: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}

	createNamespacedPolicyWithCleanup(t, policy)
	waitForPolicyReady(t, 1)

	eventGen := &framework.MockEventGen{}
	h := vpol.New(engine, testEnv.ContextProvider, nil, false, eventGen)

	ctx := framework.ContextWithPolicies(context.Background(), "ignore-on-validation-error")
	resp := h.ValidateNamespaced(ctx, logr.Discard(), framework.PodAdmissionRequest("test-pod", "fp-ignore", []byte(`{
		"apiVersion": "v1", "kind": "Pod",
		"metadata": {"name": "test-pod", "namespace": "fp-ignore"},
		"spec": {"containers": [{"name": "app", "image": "nginx"}]}
	}`)), "", time.Now())

	assert.True(t, resp.Allowed, "failurePolicy=Ignore should allow when a namespaced policy errors at runtime")
	assert.NotEmpty(t, eventGen.GetEvents(), "the ignored error should still be reported through events")
}
