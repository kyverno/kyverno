package utils

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// A CEL rule skipped by a CEL exception must still produce a PolicySkipped event
// from the background report scanner path, not just the admission path.
func TestGenerateExceptionEvents_CELRuleSkippedByCELException(t *testing.T) {
	policy := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	})
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "mypod",
			},
		},
	}
	exception := engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "skip-labels", Namespace: "default"},
	})
	er := engineapi.NewEngineResponse(resource, policy, nil).
		WithPolicyResponse(engineapi.PolicyResponse{
			Rules: []engineapi.RuleResponse{
				*engineapi.RuleSkip("require-labels", engineapi.Validation, "rule skipped", nil).
					WithExceptions([]engineapi.GenericException{exception}),
			},
		})

	events := generateExceptionEvents(er)

	assert.Len(t, events, 2)
	assert.Equal(t, event.PolicySkipped, events[0].Reason)
	assert.Equal(t, "CELPolicyException", events[0].Regarding.Kind)
	assert.Equal(t, "skip-labels", events[0].Regarding.Name)
	assert.Equal(t, event.PolicySkipped, events[1].Reason)
	assert.Equal(t, "test-policy", events[1].Regarding.Name)
}
