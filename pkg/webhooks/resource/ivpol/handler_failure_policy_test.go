package ivpol

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type forceIgnoreToggles struct {
	toggle.Toggles
}

func (forceIgnoreToggles) ForceFailurePolicyIgnore() bool { return true }

func testPolicy(name string, failurePolicy *admissionregistrationv1.FailurePolicyType, actions []admissionregistrationv1.ValidationAction, rule engineapi.RuleResponse) eval.ImageVerifyPolicyResponse {
	return eval.ImageVerifyPolicyResponse{
		Actions: sets.New(actions...),
		Policy: &policiesv1beta1.ImageValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       policiesv1beta1.ImageValidatingPolicySpec{FailurePolicy: failurePolicy},
		},
		Result: rule,
	}
}

func TestValidationResponse_FailurePolicy(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	errored := *engineapi.RuleError("verification", engineapi.ImageVerify, "error", errors.New("connection reset by peer reaching registry"), nil)
	failed := *engineapi.RuleFail("verification", engineapi.ImageVerify, "image signature verification failed", nil)
	refusedException := *engineapi.RuleError("verification", engineapi.ImageVerify, "failed to evaluate compensating controls of policy exception default/polex", errors.New("no such key: ticket"), nil).
		WithExceptions([]engineapi.GenericException{engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Name: "polex", Namespace: "default"}})})

	tests := []struct {
		name         string
		policies     []eval.ImageVerifyPolicyResponse
		forceIgnore  bool
		wantAllowed  bool
		wantWarnings int
	}{
		{
			name:        "fail blocks on runtime error",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)},
			wantAllowed: false,
		},
		{
			name:        "unset defaults to fail and blocks on runtime error",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", nil, deny, errored)},
			wantAllowed: false,
		},
		{
			name:        "ignore allows on runtime error",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, errored)},
			wantAllowed: true,
		},
		{
			name:        "ignore still blocks an image verification that evaluates to false",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, failed)},
			wantAllowed: false,
		},
		{
			name:        "ignore still blocks when a refused exception errors",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, refusedException)},
			wantAllowed: false,
		},
		{
			name:        "forceFailurePolicyIgnore overrides fail on runtime error",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)},
			forceIgnore: true,
			wantAllowed: true,
		},
		{
			name:        "forceFailurePolicyIgnore does not hide a failed verification",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, failed)},
			forceIgnore: true,
			wantAllowed: false,
		},
		{
			name: "ignore and fail policies both erroring",
			policies: []eval.ImageVerifyPolicyResponse{
				testPolicy("ignored", &ignore, deny, errored),
				testPolicy("enforced", &fail, deny, errored),
			},
			wantAllowed: false,
		},
		{
			name:         "ignore with warn still warns on runtime error",
			policies:     []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny, admissionregistrationv1.Warn}, errored)},
			wantAllowed:  true,
			wantWarnings: 1,
		},
		{
			name:        "audit only never blocks",
			policies:    []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, errored)},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.forceIgnore {
				ctx = toggle.NewContext(ctx, forceIgnoreToggles{toggle.FromContext(ctx)})
			}
			request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
			resp := (&handler{}).validationResponse(ctx, request, eval.ImageVerifyEngineResponse{Policies: tt.policies})
			assert.Equal(t, tt.wantAllowed, resp.Allowed)
			assert.Len(t, resp.Warnings, tt.wantWarnings)
		})
	}
}

func TestValidationResponse_OnlyEnforcedErrorsInMessage(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	errored := *engineapi.RuleError("verification", engineapi.ImageVerify, "error", errors.New("connection timed out"), nil)
	response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{
		testPolicy("ignored", &ignore, deny, errored),
		testPolicy("enforced", &fail, deny, errored),
	}}
	request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
	resp := (&handler{}).validationResponse(context.Background(), request, response)
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Policy enforced error")
	assert.NotContains(t, resp.Result.Message, "Policy ignored error")
}

func TestMutationResponse_FailurePolicy(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	errored := *engineapi.RuleError("mutate-digest", engineapi.ImageVerify, "failed to resolve image digest: 429 Too Many Requests", errors.New("rate limited"), nil)

	t.Run("fail blocks mutation on runtime error", func(t *testing.T) {
		t.Parallel()
		request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
		resp := (&handler{}).mutationResponse(context.Background(), request, eval.ImageVerifyEngineResponse{
			Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)},
		}, nil)
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "Policy p error")
	})

	t.Run("ignore allows mutation on runtime error", func(t *testing.T) {
		t.Parallel()
		request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
		resp := (&handler{}).mutationResponse(context.Background(), request, eval.ImageVerifyEngineResponse{
			Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, errored)},
		}, nil)
		assert.True(t, resp.Allowed)
	})

	t.Run("forceFailurePolicyIgnore overrides fail in mutation", func(t *testing.T) {
		t.Parallel()
		ctx := toggle.NewContext(context.Background(), forceIgnoreToggles{toggle.FromContext(context.Background())})
		request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
		resp := (&handler{}).mutationResponse(ctx, request, eval.ImageVerifyEngineResponse{
			Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)},
		}, nil)
		assert.True(t, resp.Allowed)
	})
}

func TestIsBlocked_FailurePolicy(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	auditOnly := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}
	errored := *engineapi.RuleError("verification", engineapi.ImageVerify, "error", errors.New("connection timed out"), nil)
	failed := *engineapi.RuleFail("verification", engineapi.ImageVerify, "signature invalid", nil)
	refusedException := *engineapi.RuleError("verification", engineapi.ImageVerify, "exception refused", errors.New("bad exception"), nil).
		WithExceptions([]engineapi.GenericException{engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Name: "polex", Namespace: "default"}})})

	t.Run("fail blocks on error", func(t *testing.T) {
		t.Parallel()
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)}}
		assert.True(t, isBlocked(context.Background(), response))
	})

	t.Run("ignore does not block on runtime error", func(t *testing.T) {
		t.Parallel()
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, errored)}}
		assert.False(t, isBlocked(context.Background(), response))
	})

	t.Run("ignore still blocks on verification failure", func(t *testing.T) {
		t.Parallel()
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, failed)}}
		assert.True(t, isBlocked(context.Background(), response))
	})

	t.Run("ignore still blocks on refused exception error", func(t *testing.T) {
		t.Parallel()
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &ignore, deny, refusedException)}}
		assert.True(t, isBlocked(context.Background(), response))
	})

	t.Run("forceIgnore does not block on error", func(t *testing.T) {
		t.Parallel()
		ctx := toggle.NewContext(context.Background(), forceIgnoreToggles{toggle.FromContext(context.Background())})
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, errored)}}
		assert.False(t, isBlocked(ctx, response))
	})

	t.Run("forceIgnore still blocks on refused exception error", func(t *testing.T) {
		t.Parallel()
		ctx := toggle.NewContext(context.Background(), forceIgnoreToggles{toggle.FromContext(context.Background())})
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, deny, refusedException)}}
		assert.True(t, isBlocked(ctx, response))
	})

	t.Run("audit action only does not block", func(t *testing.T) {
		t.Parallel()
		response := eval.ImageVerifyEngineResponse{Policies: []eval.ImageVerifyPolicyResponse{testPolicy("p", &fail, auditOnly, errored)}}
		assert.False(t, isBlocked(context.Background(), response))
	})
}
