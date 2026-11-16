package engine

import (
	"context"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestHandle_RequestBudgetChargesThroughTheRealCompiler proves the shared per-request CEL
// cost budget (pkg/cel/requestbudget) actually charges when a policy is compiled and
// evaluated the way production does it -- compiler.Compile -> compiler.CompileValidation ->
// this package's NewProgram (pkg/cel/compiler/compiler.go), never pkg/cel/compiler.Compile
// or NewProgram called directly by a test. That distinction matters: those two functions
// have no other production caller (every real Compile* helper now routes its own env.Program
// call through NewProgram internally, but a test that built its env.Program by hand would
// prove nothing about whether that routing survived).
//
// The policy has 15 validations, each a real string-concat allocation just under the
// per-program limit (the same shape as TestRequestBudgetBoundsLinearGrowthAcrossVariables in
// pkg/cel/compiler), all evaluating true so the engine's validation loop
// (vpol/compiler/policy.go) runs every one of them in order on one shared ctx instead of
// stopping at the first failure. Before Handle wired requestbudget.Ensure, all 15 would
// complete (each individually under the per-program limit); with it, evaluation is refused
// partway through.
func TestHandle_RequestBudgetChargesThroughTheRealCompiler(t *testing.T) {
	const blobSize = 2_200_000 // see pkg/cel/compiler/requestbudget_test.go's nearLimitBlobSize
	const numValidations = 15  // requestbudget's default 10,000,000 budget / ~880,000 per validation ~= 11

	validations := make([]admissionregistrationv1.Validation, numValidations)
	for i := range validations {
		validations[i] = admissionregistrationv1.Validation{
			Expression: `(object.blob + object.blob).size() >= 0`,
			Message:    "always true; only here to spend cost",
		}
	}
	policy := buildJSONPolicy("test-request-budget", validations)

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	eng := NewEngine(provider, nil, nil)

	payload := &unstructured.Unstructured{Object: map[string]any{
		"name": "allowed",
		"blob": strings.Repeat("b", blobSize),
	}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	rule := resp.Policies[0].Rules[0]
	if rule.Status() != engineapi.RuleStatusError {
		t.Fatalf("expected the shared request budget to refuse before all %d validations completed (status=%q message=%q) -- "+
			"if this is failing, requestbudget.Ensure/Charge is not reaching a program built by the real vpol compiler",
			numValidations, rule.Status(), rule.Message())
	}
	t.Logf("Handle refused: status=%q message=%q", rule.Status(), rule.Message())
}
