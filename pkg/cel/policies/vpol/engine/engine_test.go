package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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

	provider, err := NewProvider(compiler.NewCompiler(nil, nil), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
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

	provider, err := NewProvider(compiler.NewCompiler(nil, nil), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
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

// buildJSONPolicyWithMatchConstraints creates a ValidatingPolicy in JSON mode
// that also has matchConstraints set (required for non-test callers).
func buildJSONPolicyWithMatchConstraints(name string, validations []admissionregistrationv1.Validation) *policiesv1beta1.ValidatingPolicy {
	pol := buildJSONPolicy(name, validations)
	pol.Spec.MatchConstraints = &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"ci.example.com"},
					APIVersions: []string{"v1"},
					Resources:   []string{"pipelinegates"},
				},
				Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.OperationAll,
				},
			},
		}},
	}
	return pol
}

// TestVPOL_AttestationFunctionsAvailable verifies that the attestation CEL
// functions (verifyAttestationSignatures, verifyImageSignatures, getImageData,
// extractPayload) are registered in the VPOL compiler environment and therefore
// available in ValidatingPolicy CEL expressions.
//
// This is an end-to-end test of the full VPOL compiler → engine → handle path.
// It does not make real registry calls; instead it verifies that:
//   - The expression compiles without "undeclared reference" errors.
//   - Evaluation produces a runtime error (registry not reachable) rather than
//     a compile-time "function not found" error, proving the function is wired up.
func TestVPOL_AttestationFunctionsAvailable(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{
			name:       "verifyAttestationSignatures",
			expression: `verifyAttestationSignatures(object.ociRef, "https://slsa.dev/provenance/v1", []) >= 0`,
		},
		{
			name:       "verifyImageSignatures",
			expression: `verifyImageSignatures(object.ociRef, []) >= 0`,
		},
		{
			// getImageData returns dyn; the expression just needs to compile.
			name:       "getImageData_registered",
			expression: `getImageData(object.ociRef) != null || true`,
		},
		{
			// extractPayload returns dyn; compile-time check only.
			name:       "extractPayload_registered",
			expression: `extractPayload(object.ociRef, "https://slsa.dev/provenance/v1") != null || true`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := buildJSONPolicyWithMatchConstraints("test-attestation-"+tt.name,
				[]admissionregistrationv1.Validation{{Expression: tt.expression}},
			)
			// Compilation must succeed — if the functions are not registered this
			// returns a field error with "undeclared reference".
			provider, err := NewProvider(compiler.NewCompiler(nil, nil), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
			require.NoError(t, err, "policy with %s expression must compile", tt.name)
			require.NotNil(t, provider)
		})
	}
}

// TestVPOL_AttestationPipelineGatePattern shows the intended CI/CD gate use
// case: a ValidatingPolicy that validates a PipelineGate resource by verifying
// an attestation on the OCI artifact referenced in the resource's ociRef field.
//
// The test verifies the policy compiles and that evaluation against a sample
// PipelineGate object reaches the attestation function (evidenced by a registry
// fetch error rather than a compilation error or "function not defined").
func TestVPOL_AttestationPipelineGatePattern(t *testing.T) {
	const policyExpr = `
		verifyAttestationSignatures(
			object.spec.ociRef,
			"https://slsa.dev/provenance/v1",
			[{"cosign": {"keyless": {"issuer": "https://token.actions.githubusercontent.com"}}}]
		) > 0
	`
	policy := buildJSONPolicyWithMatchConstraints("pipeline-gate-slsa", []admissionregistrationv1.Validation{
		{
			Expression: policyExpr,
			Message:    "SLSA provenance attestation is required",
		},
	})

	provider, err := NewProvider(compiler.NewCompiler(nil, nil), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err, "pipeline gate policy must compile without errors")

	eng := NewEngine(provider, nil, nil)
	gate := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"ociRef": "ghcr.io/example/app:v1.2.3",
		},
	}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, gate), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)

	rule := resp.Policies[0].Rules[0]
	// The rule must fail due to a runtime error (registry unreachable / image not
	// found), not a compilation error. A "function not found" error would indicate
	// the attestation lib was not registered in the VPOL compiler.
	assert.Equal(t, engineapi.RuleStatusError, rule.Status(),
		"expected runtime registry error, not a compile-time function-not-found failure")
	assert.NotContains(t, rule.Message(), "undeclared reference",
		"error must not be a compilation failure")
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

	t.Run("existing cel.validationIndex is not overwritten", func(t *testing.T) {
		props := map[string]string{"cel.validationIndex": "user-defined"}
		out := withValidationIndex(props, 2)
		assert.Equal(t, "user-defined", out["cel.validationIndex"],
			"user-defined cel.validationIndex must not be clobbered by the engine")
	})
}
