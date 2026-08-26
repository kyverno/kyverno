package celexception

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	validation "github.com/kyverno/kyverno/pkg/validation/exception"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func newAdmissionRequest(t *testing.T, operation admissionv1.Operation, object, oldObject *policiesv1beta1.PolicyException) handlers.AdmissionRequest {
	t.Helper()
	raw, err := json.Marshal(object)
	require.NoError(t, err)
	request := handlers.AdmissionRequest{AdmissionRequest: admissionv1.AdmissionRequest{
		UID:       types.UID("test-uid"),
		Operation: operation,
		Object:    runtime.RawExtension{Raw: raw},
	}}
	if oldObject != nil {
		oldRaw, err := json.Marshal(oldObject)
		require.NoError(t, err)
		request.OldObject = runtime.RawExtension{Raw: oldRaw}
	}
	return request
}

func policyException(expression string) *policiesv1beta1.PolicyException {
	return &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "test-exception", Namespace: "default"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1alpha1.PolicyRef{{
				Name: "test-policy",
				Kind: "ValidatingPolicy",
			}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "test-condition",
				Expression: expression,
			}},
		},
	}
}

func TestValidateMatchConditions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		expression string
		allowed    bool
	}{
		{
			name:       "object",
			expression: "object.spec.containers.all(container, container.image.endsWith('@sha256:abc'))",
			allowed:    true,
		},
		{
			name:       "old object",
			expression: "oldObject.metadata.name == 'old-pod'",
			allowed:    true,
		},
		{
			name:       "request object",
			expression: "request.object.metadata.name == 'pod'",
			allowed:    true,
		},
		{
			name:       "namespace object",
			expression: "namespaceObject.metadata.name == 'default'",
			allowed:    true,
		},
		{
			name:       "undeclared receiver function",
			expression: "object.spec.containers.all(container, container.image.endsWithTYPO('@sha256:abc'))",
			allowed:    false,
		},
		{
			name:       "undeclared root variable",
			expression: "totally.unbound.variable == 'x'",
			allowed:    false,
		},
		{
			name:       "non boolean result",
			expression: "'not-a-condition'",
			allowed:    false,
		},
		{
			name:       "syntax error",
			expression: "object.metadata.name ==",
			allowed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandlers(validation.ValidationOptions{Enabled: true})
			response := handler.Validate(
				context.Background(),
				logr.Discard(),
				newAdmissionRequest(t, admissionv1.Create, policyException(tt.expression), nil),
				"",
				time.Now(),
			)

			assert.Equal(t, tt.allowed, response.Allowed)
			if !tt.allowed {
				require.NotNil(t, response.Result)
				assert.Contains(t, response.Result.Message, "spec.matchConditions[0].expression")
			}
		})
	}
}

func TestValidateUpdateRejectsNewInvalidExpression(t *testing.T) {
	t.Parallel()
	handler := NewHandlers(validation.ValidationOptions{Enabled: true})
	oldException := policyException("object.metadata.name == 'allowed-pod'")
	newException := oldException.DeepCopy()
	newException.Spec.MatchConditions[0].Expression = "object.metadata.name.endsWithTYPO('pod')"

	response := handler.Validate(
		context.Background(),
		logr.Discard(),
		newAdmissionRequest(t, admissionv1.Update, newException, oldException),
		"",
		time.Now(),
	)

	assert.False(t, response.Allowed)
	require.NotNil(t, response.Result)
	assert.Contains(t, response.Result.Message, "spec.matchConditions[0].expression")
}

func TestValidateUpdateAllowsValidExpression(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		oldExpression string
		newExpression string
	}{
		{
			name:          "valid expression remains valid",
			oldExpression: "object.metadata.name == 'allowed-pod'",
			newExpression: "object.metadata.name == 'allowed-pod'",
		},
		{
			name:          "invalid expression is corrected",
			oldExpression: "object.metadata.name.endsWithTYPO('pod')",
			newExpression: "object.metadata.name.endsWith('pod')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handler := NewHandlers(validation.ValidationOptions{Enabled: true})
			oldException := policyException(tt.oldExpression)
			newException := oldException.DeepCopy()
			newException.Spec.MatchConditions[0].Expression = tt.newExpression

			response := handler.Validate(
				context.Background(),
				logr.Discard(),
				newAdmissionRequest(t, admissionv1.Update, newException, oldException),
				"",
				time.Now(),
			)

			assert.True(t, response.Allowed)
		})
	}
}
