package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/ext"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/sdk/extensions/cel/libs/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// secretEnv is an environment with the resource library, as the policy compilers build it, over
// a fake cluster holding one Secret and one ConfigMap.
func secretEnv(t *testing.T, namespace string) *cel.Env {
	t.Helper()
	cluster := libs.NewFakeContextProvider()
	require.NoError(t, cluster.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "secrets"}, &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: "db-credentials", Namespace: "kyverno"},
		Data:       map[string][]byte{"password": []byte("s3cr3t-p4ss")},
	}))
	require.NoError(t, cluster.AddResource(schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}, &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "kyverno"},
		Data:       map[string]string{"mode": "strict"},
	}))
	env, err := cel.NewEnv(
		ext.Encoders(),
		cel.Variable("object", cel.DynType),
		cel.Variable("variables", cel.MapType(cel.StringType, cel.DynType)),
		resource.Lib(resource.Context{ContextInterface: cluster}, namespace, resource.Latest()),
	)
	require.NoError(t, err)
	return env
}

func TestInputsOf(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		expression string
		want       Inputs
	}{{
		name:       "a Secret by name",
		expression: "resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password",
		want:       Inputs{Secret: true},
	}, {
		name:       "a ConfigMap",
		expression: "resource.Get('v1', 'configmaps', 'kyverno', 'settings').data.mode",
		want:       Inputs{},
	}, {
		name:       "a list of Secrets",
		expression: "resource.List('v1', 'secrets', 'kyverno').items.size() > 0",
		want:       Inputs{Secret: true},
	}, {
		name:       "a resource that is not written out may be secrets",
		expression: "resource.Get('v1', object.kind, 'kyverno', 'db-credentials').data.password",
		want:       Inputs{Secret: true},
	}, {
		name:       "a Secret by GVR",
		expression: "resource.Get(resource.ToGVR('v1', 'secrets'), 'kyverno', 'db-credentials').data.password",
		want:       Inputs{Secret: true},
	}, {
		name:       "a ConfigMap by GVR",
		expression: "resource.Get(resource.ToGVR('v1', 'configmaps'), 'kyverno', 'settings').data.mode",
		want:       Inputs{},
	}, {
		name:       "a GVR that is not written out may be secrets",
		expression: "resource.Get(variables.gvr, 'kyverno', 'db-credentials').data.password",
		want:       Inputs{Secret: true, Variables: []string{"gvr"}},
	}, {
		name:       "a namespaced policy's Secret",
		namespace:  "kyverno",
		expression: "resource.Get('v1', 'secrets', 'db-credentials').data.password",
		want:       Inputs{Secret: true},
	}, {
		name:       "a created Secret",
		expression: "resource.Post('v1', 'secrets', 'kyverno', {'metadata': {'name': 'copy'}}).data",
		want:       Inputs{Secret: true},
	}, {
		name:       "variables, each once",
		expression: "variables.password != '' && variables['team'] == 'payments' && variables.password.size() > 3",
		want:       Inputs{Variables: []string{"password", "team"}},
	}, {
		name:       "the resource itself",
		expression: "object.metadata.name",
		want:       Inputs{},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, iss := secretEnv(t, tt.namespace).Compile(tt.expression)
			require.NoError(t, iss.Err())
			assert.Equal(t, tt.want, Build(tt.expression, ast, nil, nil).Inputs)
		})
	}
}

// TestInputsOf_ParsedOnly: an AST that was parsed but not checked has no overload IDs, and the
// resource calls are recognized by their name.
func TestInputsOf_ParsedOnly(t *testing.T) {
	env := secretEnv(t, "")
	for expression, want := range map[string]bool{
		"resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password":                 true,
		"resource.Get(resource.ToGVR('v1', 'secrets'), 'kyverno', 'db-credentials').data.password": true,
		"resource.List('v1', object.kind, 'kyverno')":                                              true,
		"resource.Get('v1', 'configmaps', 'kyverno', 'settings').data.mode":                        false,
		"resource.Get(resource.ToGVR('v1', 'configmaps'), 'kyverno', 'settings').data.mode":        false,
		"http.Get('https://example.invalid/v1/secrets').body":                                      false,
		"resource.ToGVR('v1', 'secrets').resource == 'secrets'":                                    false,
		"object.metadata.name == resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').kind":  true,
	} {
		ast, iss := env.Parse(expression)
		require.NoError(t, iss.Err())
		assert.Equal(t, want, Build(expression, ast, nil, nil).Inputs.Secret, expression)
	}
}

// TestBuild_NodeInputs: each node of the breakdown records what it reads, so a node that reads a
// Secret's value is told apart from one beside it that does not.
func TestBuild_NodeInputs(t *testing.T) {
	env := secretEnv(t, "")
	expression := "variables.password.size() > 32 && object.metadata.name != ''"
	ast, iss := env.Compile(expression)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
	require.NoError(t, err)
	out, details, err := prg.ContextEval(context.TODO(), map[string]any{
		"object":    map[string]any{"metadata": map[string]any{"name": "web"}},
		"variables": map[string]any{"password": "s3cr3t-p4ss"},
	})
	require.NoError(t, err)

	et := Build(expression, ast, out, details)
	assert.Equal(t, Inputs{Variables: []string{"password"}}, et.Inputs)
	inputs := map[string]Inputs{}
	for _, n := range et.Nodes {
		inputs[n.Expression] = n.Inputs
	}
	assert.Equal(t, Inputs{Variables: []string{"password"}}, inputs["variables.password"])
	assert.Equal(t, Inputs{Variables: []string{"password"}}, inputs["variables.password.size()"])
	assert.Equal(t, Inputs{}, inputs["object.metadata.name"])
}

// evaluated builds the trace of expression, evaluated over the fake cluster of secretEnv with
// the given variables.
func evaluated(t *testing.T, expression string, variables map[string]any) ExpressionTrace {
	t.Helper()
	env := secretEnv(t, "")
	ast, iss := env.Compile(expression)
	require.NoError(t, iss.Err())
	prg, err := env.Program(ast, cel.EvalOptions(cel.OptTrackState))
	require.NoError(t, err)
	out, details, err := prg.ContextEval(context.TODO(), map[string]any{
		"object":    map[string]any{"metadata": map[string]any{"name": "web"}},
		"variables": variables,
	})
	if out == nil && err != nil {
		out = types.WrapErr(err)
	}
	return Build(expression, ast, out, details)
}

// secretDecision is a Pod decision of a policy that reads a Secret's password into a variable,
// decodes it into another, reads a ConfigMap into a third, and checks the password.
func secretDecision(t *testing.T) *Decision {
	t.Helper()
	password := evaluated(t, "resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password", nil)
	require.Equal(t, "czNjcjN0LXA0c3M=", password.Result, "the fake Secret's data, as resource.Get returns it")
	// a variable is traced when it is first read, so one that reads another comes first
	decoded := evaluated(t, "string(base64.decode(variables.password))", map[string]any{"password": password.Result})
	mode := evaluated(t, "resource.Get('v1', 'configmaps', 'kyverno', 'settings').data.mode", nil)
	check := evaluated(t, "variables.mode == 'strict' && variables.decoded.size() > 32", map[string]any{"decoded": decoded.Result, "mode": mode.Result})
	return &Decision{
		PolicyName:   "check-password",
		PolicyKind:   "ValidatingPolicy",
		ResourceKind: "Pod",
		ResourceName: "web",
		Variables: []NamedExpressionTrace{
			{Name: "decoded", ExpressionTrace: decoded},
			{Name: "password", ExpressionTrace: password},
			{Name: "mode", ExpressionTrace: mode},
		},
		Verdict: VerdictTrace{Status: VerdictFail, Message: "the password is too short", ExpressionTrace: check},
	}
}

// TestRedaction_SecretValuesAreHidden: a value read from a Secret, or worked out from one, is
// hidden in both the CLI output and the admission log of a decision on another kind, while the
// values that do not come from a Secret are still shown.
func TestRedaction_SecretValuesAreHidden(t *testing.T) {
	d := secretDecision(t)
	var rendered bytes.Buffer
	Render(&rendered, d)
	entry, _ := logged(t, d)
	logLine, err := json.Marshal(entry)
	require.NoError(t, err)

	for name, output := range map[string]string{"render": rendered.String(), "log": string(logLine)} {
		t.Run(name, func(t *testing.T) {
			assert.NotContains(t, output, "czNjcjN0LXA0c3M=", "the Secret's data")
			assert.NotContains(t, output, "s3cr3t-p4ss", "the Secret's data, decoded")
			assert.Contains(t, output, "strict", "a ConfigMap's value is not hidden")
			assert.Contains(t, output, "the password is too short", "the policy's own message is kept")
		})
	}
	t.Log(rendered.String())

	assert.Equal(t, redacted, entry.Variables[0].Result, "decoded reads password, which reads a Secret")
	assert.Equal(t, redacted, entry.Variables[1].Result)
	assert.Equal(t, "strict", entry.Variables[2].Result)
	assert.Equal(t, "false", entry.Verdict.Result, "true and false are kept")
	evaluated := map[string]string{}
	for _, n := range entry.Verdict.Evaluated {
		evaluated[n.Expression] = n.Value
	}
	assert.Equal(t, redacted, evaluated["variables.decoded"])
	assert.Equal(t, redacted, evaluated["variables.decoded.size()"], "a value worked out from a Secret is hidden too")
	assert.Equal(t, "strict", evaluated["variables.mode"])
}

// TestRedaction_SecretErrorsAreHidden: an error can quote the value it failed on, so the error of
// an expression that reads a Secret is hidden, and so is the message of the ERROR verdict it
// leads to.
func TestRedaction_SecretErrorsAreHidden(t *testing.T) {
	failing := evaluated(t, "timestamp(resource.Get('v1', 'secrets', 'kyverno', 'db-credentials').data.password) < timestamp('2026-01-01T00:00:00Z')", nil)
	require.Contains(t, failing.Result, "czNjcjN0LXA0c3M=", "the error quotes the Secret's data")
	d := &Decision{
		PolicyName:   "check-expiry",
		PolicyKind:   "ImageValidatingPolicy",
		ResourceKind: "Pod",
		ResourceName: "web",
		Verdict:      VerdictTrace{Status: VerdictError, Message: failing.Result, ExpressionTrace: failing},
	}
	var rendered bytes.Buffer
	Render(&rendered, d)
	entry, _ := logged(t, d)
	logLine, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.NotContains(t, rendered.String(), "czNjcjN0LXA0c3M=")
	assert.NotContains(t, string(logLine), "czNjcjN0LXA0c3M=")
	assert.Equal(t, redacted, entry.Verdict.Result)
	assert.Equal(t, redacted, entry.Verdict.Message)
	t.Log(rendered.String())

	// an error that has nothing to do with a Secret is kept
	other := evaluated(t, "timestamp(resource.Get('v1', 'configmaps', 'kyverno', 'settings').data.mode) < timestamp('2026-01-01T00:00:00Z')", nil)
	d.Verdict = VerdictTrace{Status: VerdictError, Message: other.Result, ExpressionTrace: other}
	entry, _ = logged(t, d)
	assert.Contains(t, entry.Verdict.Message, "strict")
}

// TestRender_SecretResourceValuesAreHidden: the CLI hides the values of a decision on a Secret,
// as the admission log already did (see TestLogValues_SecretValuesAreNotLogged).
func TestRender_SecretResourceValuesAreHidden(t *testing.T) {
	const password = "hunter2-do-not-print"
	d := &Decision{
		PolicyName:   "secret-rules",
		PolicyKind:   "ValidatingPolicy",
		ResourceKind: "Secret",
		ResourceName: "db-credentials",
		Variables:    []NamedExpressionTrace{{Name: "password", ExpressionTrace: ExpressionTrace{Source: "object.data.password", Result: password}}},
		Verdict: VerdictTrace{
			Status: VerdictFail,
			ExpressionTrace: ExpressionTrace{
				Source: "variables.password.size() > 32",
				Result: "false",
				Nodes: []NodeTrace{
					{Expression: "variables.password.size()", Value: "18"},
					{Expression: "timestamp(variables.password)", Error: `invalid RFC 3339 timestamp "` + password + `"`},
				},
			},
		},
	}
	var rendered bytes.Buffer
	Render(&rendered, d)
	assert.NotContains(t, rendered.String(), password)
	assert.NotContains(t, rendered.String(), "18")
	assert.Contains(t, rendered.String(), "variables.password.size() > 32  ->  false")
}
