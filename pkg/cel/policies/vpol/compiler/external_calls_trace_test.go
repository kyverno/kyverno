package compiler

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// TestCompile_NoTrackingTwinForCallsThatMustNotRunTwice: an expression that calls a function with
// an external effect or a non-repeatable result gets no explain-only tracking twin, so --explain
// never calls it a second time; an expression that only reads its inputs keeps its twin.
func TestCompile_NoTrackingTwinForCallsThatMustNotRunTwice(t *testing.T) {
	tests := []struct {
		expression string
		want       string
	}{
		{`http.Get('http://example.invalid').ok == true`, "http.Get"},
		{`http.Post('http://example.invalid', {'a': 1}).ok == true`, "http.Post"},
		{`http.Client('').Post('http://example.invalid', {'a': 1}).ok == true`, "http.Post"},
		{`resource.Get('v1', 'configmaps', 'default', 'x').data.a == 'b'`, "resource.Get"},
		{`resource.List('v1', 'configmaps', 'default').items.size() > 0`, "resource.List"},
		{`resource.Post('v1', 'configmaps', 'default', {'a': 1}).metadata.name != ''`, "resource.Post"},
		{`image.GetMetadata('nginx:1.27').config != null`, "image.GetMetadata"},
		{`random() != ''`, "random"},
		{`time.now() > timestamp('2020-01-01T00:00:00Z')`, "time.now"},
		{`object.metadata.namespace == 'prod'`, ""},
		{`has(object.metadata.labels) && 'team' in object.metadata.labels`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.expression, func(t *testing.T) {
			policy := buildTracePolicy(admissionregistrationv1.Validation{Expression: tt.expression})
			// the same expression as a match condition and as a variable, which compile separately
			policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "condition", Expression: tt.expression}}
			policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "value", Expression: tt.expression}}
			p, errs := NewCompilerWithTrace(true).Compile(policy, nil)
			require.Empty(t, errs)

			assert.Equal(t, tt.want, compiler.NotRepeatable(p.validations[0].AST))
			hasTwin := tt.want == ""
			assert.Equal(t, hasTwin, p.validations[0].Traced != nil, "validation")
			assert.Equal(t, hasTwin, p.tracedMatchConditions[0].Traced != nil, "match condition")
			assert.Equal(t, hasTwin, p.tracedVariables["value"].Traced != nil, "variable")
			assert.NotNil(t, p.validations[0].AST, "the AST is kept either way, so the trace still shows the expression")
		})
	}
}

// TestEvaluate_TracingNotesWhyAnExpressionHasNoBreakdown: an expression that calls random() is not
// re-run to explain it, so its trace has no per-node breakdown; the trace says why instead of
// leaving the breakdown silently empty. (That the call itself runs only once is checked in
// pkg/cel/compiler, where every policy type builds its tracking twins.)
func TestEvaluate_TracingNotesWhyAnExpressionHasNoBreakdown(t *testing.T) {
	result := compileAndEvaluate(t, true, buildTracePolicy(admissionregistrationv1.Validation{
		Expression: `random('[a-z]{8}') == 'never-matches-this'`,
		Message:    "random value did not match",
	}), podObject("prod", map[string]any{"team": "a"}))
	require.NotNil(t, result)
	require.NotNil(t, result.Trace)
	assert.Equal(t, trace.VerdictFail, result.Trace.Verdict.Status)
	assert.Equal(t, "false", result.Trace.Verdict.Result)
	assert.Empty(t, result.Trace.Verdict.Nodes)
	assert.Equal(t, "it calls random, which is not run a second time", result.Trace.Verdict.NoBreakdown)

	plain := compileAndEvaluate(t, true, buildTracePolicy(admissionregistrationv1.Validation{
		Expression: `object.metadata.namespace == 'staging'`,
	}), podObject("prod", nil))
	require.NotNil(t, plain.Trace)
	assert.Empty(t, plain.Trace.Verdict.NoBreakdown)
	assert.NotEmpty(t, plain.Trace.Verdict.Nodes, "an expression that only reads its inputs keeps its breakdown")
}
