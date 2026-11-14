package policies

import (
	"testing"

	"github.com/google/cel-go/cel"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsKyvernoEvaluationMode(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{mode: "", want: true},
		{mode: "Kubernetes", want: true},
		{mode: "JSON", want: true},
		{mode: "Envoy", want: false},
		{mode: "HTTP", want: false},
		{mode: "kubernetes", want: false},
		{mode: "Unknown", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			assert.Equal(t, tt.want, IsKyvernoEvaluationMode(tt.mode))
		})
	}
}

func TestIsKyvernoOwned(t *testing.T) {
	vpol := func(mode string) policiesv1beta1.ValidatingPolicySpec {
		return policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{Mode: mode},
		}
	}
	assert.True(t, IsKyvernoOwned(policiesv1beta1.ValidatingPolicySpec{}))
	assert.True(t, IsKyvernoOwned(vpol("JSON")))
	assert.False(t, IsKyvernoOwned(vpol("Envoy")))
	assert.False(t, IsKyvernoOwned(vpol("HTTP")))
}

func evalMatchCondition(t *testing.T, expression string, object map[string]any) bool {
	t.Helper()
	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	require.NoError(t, err)
	ast, issues := env.Compile(expression)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)
	out, _, err := program.Eval(map[string]any{"object": object})
	require.NoError(t, err)
	return out.Value().(bool)
}

func TestPolicyEvaluationModeMatchCondition(t *testing.T) {
	withMode := func(mode string) map[string]any {
		return map[string]any{"spec": map[string]any{"evaluation": map[string]any{"mode": mode}}}
	}
	tests := []struct {
		name   string
		object map[string]any
		want   bool
	}{
		{name: "no spec", object: map[string]any{}, want: true},
		{name: "legacy policy", object: map[string]any{"spec": map[string]any{"rules": []any{}}}, want: true},
		{name: "no mode", object: map[string]any{"spec": map[string]any{"evaluation": map[string]any{}}}, want: true},
		{name: "empty mode", object: withMode(""), want: true},
		{name: "Kubernetes", object: withMode("Kubernetes"), want: true},
		{name: "JSON", object: withMode("JSON"), want: true},
		{name: "Envoy", object: withMode("Envoy"), want: false},
		{name: "HTTP", object: withMode("HTTP"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalMatchCondition(t, PolicyEvaluationModeMatchCondition().Expression, tt.object))
		})
	}
}

func TestExceptionEvaluationModeMatchCondition(t *testing.T) {
	withMode := func(mode string) map[string]any {
		return map[string]any{"spec": map[string]any{"evaluationMode": mode}}
	}
	tests := []struct {
		name   string
		object map[string]any
		want   bool
	}{
		{name: "no mode", object: map[string]any{"spec": map[string]any{}}, want: true},
		{name: "Kubernetes", object: withMode("Kubernetes"), want: true},
		{name: "JSON", object: withMode("JSON"), want: true},
		{name: "Envoy", object: withMode("Envoy"), want: false},
		{name: "HTTP", object: withMode("HTTP"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, evalMatchCondition(t, ExceptionEvaluationModeMatchCondition().Expression, tt.object))
		})
	}
}

func TestIsKyvernoOwnedException(t *testing.T) {
	polex := func(mode string) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			Spec: policiesv1beta1.PolicyExceptionSpec{EvaluationMode: mode},
		}
	}
	assert.True(t, IsKyvernoOwnedException(polex("")))
	assert.True(t, IsKyvernoOwnedException(polex("Kubernetes")))
	assert.False(t, IsKyvernoOwnedException(polex("HTTP")))
}
