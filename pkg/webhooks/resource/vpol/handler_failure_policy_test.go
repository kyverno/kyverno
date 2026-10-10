package vpol

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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

func testPolicy(name string, failurePolicy *admissionregistrationv1.FailurePolicyType, actions []admissionregistrationv1.ValidationAction, rules ...engineapi.RuleResponse) celengine.ValidatingPolicyResponse {
	return celengine.ValidatingPolicyResponse{
		Actions: sets.New(actions...),
		Policy: &policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       policiesv1beta1.ValidatingPolicySpec{FailurePolicy: failurePolicy},
		},
		Rules: rules,
	}
}

func TestAdmissionResponse_FailurePolicy(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	errored := *engineapi.RuleError("validation", engineapi.Validation, "error", errors.New("no such key: team"), nil)
	failed := *engineapi.RuleFail("validation", engineapi.Validation, "team label is required", nil)
	refusedException := *engineapi.RuleError("validation", engineapi.Validation, "failed to evaluate compensating controls of policy exception default/polex", errors.New("no such key: ticket"), nil).
		WithExceptions([]engineapi.GenericException{engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Name: "polex", Namespace: "default"}})})
	tests := []struct {
		name         string
		policies     []celengine.ValidatingPolicyResponse
		forceIgnore  bool
		wantAllowed  bool
		wantWarnings int
	}{{
		name:        "fail blocks on runtime error",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &fail, deny, errored)},
		wantAllowed: false,
	}, {
		name:        "unset defaults to fail and blocks on runtime error",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", nil, deny, errored)},
		wantAllowed: false,
	}, {
		name:        "ignore allows on runtime error",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &ignore, deny, errored)},
		wantAllowed: true,
	}, {
		name:        "ignore still blocks a validation that evaluates to false",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &ignore, deny, failed)},
		wantAllowed: false,
	}, {
		name:        "ignore still blocks when a refused exception errors",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &ignore, deny, refusedException)},
		wantAllowed: false,
	}, {
		name:        "forceFailurePolicyIgnore overrides fail on runtime error",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &fail, deny, errored)},
		forceIgnore: true,
		wantAllowed: true,
	}, {
		name:        "forceFailurePolicyIgnore does not hide a failed validation",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &fail, deny, failed)},
		forceIgnore: true,
		wantAllowed: false,
	}, {
		name: "ignore and fail policies both erroring",
		policies: []celengine.ValidatingPolicyResponse{
			testPolicy("ignored", &ignore, deny, errored),
			testPolicy("enforced", &fail, deny, errored),
		},
		wantAllowed: false,
	}, {
		name:         "ignore with warn still warns on runtime error",
		policies:     []celengine.ValidatingPolicyResponse{testPolicy("p", &ignore, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny, admissionregistrationv1.Warn}, errored)},
		wantAllowed:  true,
		wantWarnings: 1,
	}, {
		name:        "audit only never blocks",
		policies:    []celengine.ValidatingPolicyResponse{testPolicy("p", &fail, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, errored)},
		wantAllowed: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.forceIgnore {
				ctx = toggle.NewContext(ctx, forceIgnoreToggles{toggle.FromContext(ctx)})
			}
			request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
			resp := (&handler{}).admissionResponse(ctx, request, celengine.EngineResponse{Policies: tt.policies})
			assert.Equal(t, tt.wantAllowed, resp.Allowed)
			assert.Len(t, resp.Warnings, tt.wantWarnings)
		})
	}
}

func TestAdmissionResponse_OnlyEnforcedErrorsInMessage(t *testing.T) {
	t.Parallel()
	fail := admissionregistrationv1.Fail
	ignore := admissionregistrationv1.Ignore
	deny := []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}
	errored := *engineapi.RuleError("validation", engineapi.Validation, "error", errors.New("no such key: team"), nil)
	response := celengine.EngineResponse{Policies: []celengine.ValidatingPolicyResponse{
		testPolicy("ignored", &ignore, deny, errored),
		testPolicy("enforced", &fail, deny, errored),
	}}
	request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{UID: "test-uid"}}
	resp := (&handler{}).admissionResponse(context.Background(), request, response)
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "Policy enforced error")
	assert.NotContains(t, resp.Result.Message, "Policy ignored error")
}
