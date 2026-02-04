package validation

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandleValidationEnforce_EmptyPolicies(t *testing.T) {
	h := &validationHandler{
		log: logr.Discard(),
	}

	ctx := context.Background()
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"}}`)},
		},
	}

	// Empty policies should return allowed=true
	allowed, msg, warnings, responses := h.HandleValidationEnforce(ctx, request, nil, nil, time.Now())

	assert.True(t, allowed, "empty policies should allow request")
	assert.Empty(t, msg)
	assert.Empty(t, warnings)
	assert.Empty(t, responses)
}

func TestHandleValidationEnforce_EmptyPoliciesSlice(t *testing.T) {
	h := &validationHandler{
		log: logr.Discard(),
	}

	ctx := context.Background()
	request := handlers.AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"}}`)},
		},
	}

	// Empty slices (not nil) should also return allowed=true
	allowed, msg, warnings, responses := h.HandleValidationEnforce(ctx, request, []kyvernov1.PolicyInterface{}, []kyvernov1.PolicyInterface{}, time.Now())

	assert.True(t, allowed, "empty policies slice should allow request")
	assert.Empty(t, msg)
	assert.Empty(t, warnings)
	assert.Empty(t, responses)
}

func TestHasReportablePolicy_NilPolicies(t *testing.T) {
	result := hasReportablePolicy(nil)
	assert.False(t, result, "nil policies should return false")
}

func TestHasReportablePolicy_EmptyPolicies(t *testing.T) {
	result := hasReportablePolicy([]kyvernov1.PolicyInterface{})
	assert.False(t, result, "empty policies should return false")
}

func TestHasReportablePolicy_WithBackgroundTrue(t *testing.T) {
	bgTrue := true
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: kyvernov1.Spec{
			Background: &bgTrue,
		},
	}

	result := hasReportablePolicy([]kyvernov1.PolicyInterface{policy})
	assert.True(t, result, "policy with background=true should be reportable")
}

func TestHasReportablePolicy_MixedPolicies(t *testing.T) {
	bgTrue := true
	bgFalse := false

	reportable := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "reportable"},
		Spec:       kyvernov1.Spec{Background: &bgTrue},
	}
	nonReportable := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "non-reportable"},
		Spec:       kyvernov1.Spec{Background: &bgFalse},
	}

	// If any policy is reportable, should return true
	result := hasReportablePolicy([]kyvernov1.PolicyInterface{nonReportable, reportable})
	assert.True(t, result, "should be true if any policy is reportable")
}
