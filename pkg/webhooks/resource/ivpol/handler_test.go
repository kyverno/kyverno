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

// forceIgnoreToggles is a Toggles implementation that only forces failurePolicy
// to Ignore; every other toggle is off. validationResponse consults only
// ForceFailurePolicyIgnore.
type forceIgnoreToggles struct{}

func (forceIgnoreToggles) ProtectManagedResources() bool           { return false }
func (forceIgnoreToggles) ForceFailurePolicyIgnore() bool          { return true }
func (forceIgnoreToggles) EnableDeferredLoading() bool             { return false }
func (forceIgnoreToggles) GenerateValidatingAdmissionPolicy() bool { return false }
func (forceIgnoreToggles) GenerateMutatingAdmissionPolicy() bool   { return false }
func (forceIgnoreToggles) DumpMutatePatches() bool                 { return false }
func (forceIgnoreToggles) AutogenV2() bool                         { return false }
func (forceIgnoreToggles) AllowHTTPInNamespacedPolicies() bool     { return false }

func failurePolicyPtr(f admissionregistrationv1.FailurePolicyType) *admissionregistrationv1.FailurePolicyType {
	return &f
}

func ivpolWithFailurePolicy(name string, fp *admissionregistrationv1.FailurePolicyType) *policiesv1beta1.ImageValidatingPolicy {
	return &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       policiesv1beta1.ImageValidatingPolicySpec{FailurePolicy: fp},
	}
}

func denyResponse(result *engineapi.RuleResponse, fp *admissionregistrationv1.FailurePolicyType) eval.ImageVerifyEngineResponse {
	return eval.ImageVerifyEngineResponse{
		Policies: []eval.ImageVerifyPolicyResponse{
			{
				Policy:  ivpolWithFailurePolicy("test-policy", fp),
				Actions: sets.New(admissionregistrationv1.Deny),
				Result:  *result,
			},
		},
	}
}

func TestValidationResponse_FailurePolicyForErrors(t *testing.T) {
	evalErr := errors.New("verification could not be performed")

	tests := []struct {
		name          string
		result        *engineapi.RuleResponse
		failurePolicy *admissionregistrationv1.FailurePolicyType
		forceIgnore   bool
		wantAllowed   bool
		wantWarnings  int
	}{
		{
			name:          "error under Deny with failurePolicy Ignore is admitted with a warning",
			result:        engineapi.RuleError("test-rule", engineapi.Validation, "image verification failed", evalErr, nil),
			failurePolicy: failurePolicyPtr(admissionregistrationv1.Ignore),
			wantAllowed:   true,
			wantWarnings:  1,
		},
		{
			name:          "error under Deny with failurePolicy Fail is denied",
			result:        engineapi.RuleError("test-rule", engineapi.Validation, "image verification failed", evalErr, nil),
			failurePolicy: failurePolicyPtr(admissionregistrationv1.Fail),
			wantAllowed:   false,
			wantWarnings:  0,
		},
		{
			name:          "error under Deny defaults to Fail (nil failurePolicy) and is denied",
			result:        engineapi.RuleError("test-rule", engineapi.Validation, "image verification failed", evalErr, nil),
			failurePolicy: nil,
			wantAllowed:   false,
			wantWarnings:  0,
		},
		{
			name:          "genuine fail under Deny always denies regardless of failurePolicy Ignore",
			result:        engineapi.RuleFail("test-rule", engineapi.Validation, "image is not signed", nil),
			failurePolicy: failurePolicyPtr(admissionregistrationv1.Ignore),
			wantAllowed:   false,
			wantWarnings:  0,
		},
		{
			name:          "error under Deny with forceFailurePolicyIgnore is admitted with a warning even when spec is Fail",
			result:        engineapi.RuleError("test-rule", engineapi.Validation, "image verification failed", evalErr, nil),
			failurePolicy: failurePolicyPtr(admissionregistrationv1.Fail),
			forceIgnore:   true,
			wantAllowed:   true,
			wantWarnings:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.forceIgnore {
				ctx = toggle.NewContext(ctx, forceIgnoreToggles{})
			}

			h := &handler{}
			request := celengine.EngineRequest{
				Request: admissionv1.AdmissionRequest{UID: "test-uid"},
			}
			response := denyResponse(tt.result, tt.failurePolicy)

			got := h.validationResponse(ctx, request, response)

			assert.Equal(t, tt.wantAllowed, got.Allowed)
			assert.Len(t, got.Warnings, tt.wantWarnings)
			if tt.wantAllowed {
				assert.Nil(t, got.Result)
			} else {
				assert.NotNil(t, got.Result)
			}
		})
	}
}

// A policy carrying both Deny and Warn actions must emit exactly one warning for
// a RuleStatusError admitted under failurePolicy Ignore, not a duplicate from
// each branch.
func TestValidationResponse_DenyAndWarn_ErrorEmitsSingleWarning(t *testing.T) {
	evalErr := errors.New("verification could not be performed")
	result := engineapi.RuleError("test-rule", engineapi.Validation, "image verification failed", evalErr, nil)

	response := eval.ImageVerifyEngineResponse{
		Policies: []eval.ImageVerifyPolicyResponse{
			{
				Policy:  ivpolWithFailurePolicy("test-policy", failurePolicyPtr(admissionregistrationv1.Ignore)),
				Actions: sets.New(admissionregistrationv1.Deny, admissionregistrationv1.Warn),
				Result:  *result,
			},
		},
	}

	h := &handler{}
	request := celengine.EngineRequest{
		Request: admissionv1.AdmissionRequest{UID: "test-uid"},
	}

	got := h.validationResponse(context.Background(), request, response)

	assert.True(t, got.Allowed)
	assert.Len(t, got.Warnings, 1)
}
