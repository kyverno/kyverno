package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// buildJSONPolicy creates a ValidatingPolicy in JSON evaluation mode with the
// given validation expressions for use in unit tests.
func buildJSONPolicy(name string, validations []admissionregistrationv1.Validation) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: validations,
		},
	}
}

func TestHandle_ValidationIndexInProperties(t *testing.T) {
	// Four expressions; only the third (index 2) fails.
	// cel.validationIndex in the response properties must be "2".
	policy := buildJSONPolicy("test-index", []admissionregistrationv1.Validation{
		{Expression: "object.name == 'allowed'", Message: "index 0: passes"},
		{Expression: "size(object.name) > 0", Message: "index 1: passes"},
		{Expression: "object.name == 'forbidden'", Message: "index 2: fails"},
		{Expression: "object.name != ''", Message: "index 3: would pass"},
	})

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)

	eng := NewEngine(provider, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "allowed"}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	rule := resp.Policies[0].Rules[0]
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Contains(t, rule.Message(), "index 2: fails")
	assert.Equal(t, "2", rule.Properties()["cel.validationIndex"],
		"cel.validationIndex must reflect the actual failing expression index, not the loop counter")
}

func TestHandle_ValidationIndexFirstExpression(t *testing.T) {
	// When the first expression fails, cel.validationIndex must be "0".
	policy := buildJSONPolicy("test-index-first", []admissionregistrationv1.Validation{
		{Expression: "object.name == 'wrong'", Message: "index 0: fails"},
		{Expression: "object.name != ''", Message: "index 1: would pass"},
	})

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)

	eng := NewEngine(provider, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "allowed"}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	rule := resp.Policies[0].Rules[0]
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "0", rule.Properties()["cel.validationIndex"])
}

func TestWithValidationIndex(t *testing.T) {
	t.Run("nil props", func(t *testing.T) {
		out := withValidationIndex(nil, 3)
		assert.Equal(t, "3", out["cel.validationIndex"])
	})

	t.Run("existing props are preserved", func(t *testing.T) {
		props := map[string]string{"existing-key": "existing-value"}
		out := withValidationIndex(props, 1)
		assert.Equal(t, "1", out["cel.validationIndex"])
		assert.Equal(t, "existing-value", out["existing-key"])
	})

	t.Run("does not mutate original map", func(t *testing.T) {
		props := map[string]string{"key": "val"}
		_ = withValidationIndex(props, 5)
		_, exists := props["cel.validationIndex"]
		assert.False(t, exists, "original props map must not be mutated")
	})
}
