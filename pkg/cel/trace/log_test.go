package trace

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// logged returns the LogEntry of LogValues(d), and its other pairs as a map.
func logged(t *testing.T, d *Decision) (LogEntry, map[string]any) {
	t.Helper()
	values := LogValues(d)
	require.Zero(t, len(values)%2, "key/value pairs")
	pairs := map[string]any{}
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		require.True(t, ok)
		pairs[key] = values[i+1]
	}
	entry, ok := pairs["trace"].(LogEntry)
	require.True(t, ok)
	return entry, pairs
}

func failedPodDecision() *Decision {
	return &Decision{
		PolicyName:   "require-team-label",
		PolicyKind:   "ValidatingPolicy",
		ResourceKind: "Pod",
		ResourceName: "web",
		Scope:        ScopeTrace{Applied: true, Reason: "matched resourceRules[0]"},
		Match:        []NamedExpressionTrace{{Name: "not-system", ExpressionTrace: ExpressionTrace{Source: "object.metadata.namespace != 'kube-system'", Result: "true"}}},
		Variables:    []NamedExpressionTrace{{Name: "team", ExpressionTrace: ExpressionTrace{Source: "object.metadata.labels.team", Result: "payments"}}},
		Verdict: VerdictTrace{
			Status:  VerdictFail,
			Message: "the team label must be platform",
			ExpressionTrace: ExpressionTrace{
				Source: "variables.team == 'platform'",
				Result: "false",
				Nodes: []NodeTrace{
					{Expression: "variables.team == 'platform'", Value: "false"},
					{Expression: "variables.team", Value: "payments"},
				},
			},
		},
	}
}

func TestLogValues(t *testing.T) {
	assert.Nil(t, LogValues(nil))

	t.Run("a failed decision keeps the breakdown of what decided", func(t *testing.T) {
		entry, pairs := logged(t, failedPodDecision())
		assert.Equal(t, "require-team-label", pairs["policy"])
		assert.Equal(t, "ValidatingPolicy", pairs["policyKind"])
		assert.Equal(t, VerdictFail, pairs["verdict"])

		assert.Equal(t, "matched resourceRules[0]", entry.Scope)
		assert.Equal(t, []LogExpression{{Name: "not-system", Expression: "object.metadata.namespace != 'kube-system'", Result: "true"}}, entry.Match)
		assert.Equal(t, []LogExpression{{Name: "team", Expression: "object.metadata.labels.team", Result: "payments"}}, entry.Variables)
		assert.Empty(t, entry.Validations, "a single validation is the verdict, it is not listed again")
		assert.Equal(t, LogExpression{
			Status:     VerdictFail,
			Expression: "variables.team == 'platform'",
			Result:     "false",
			Message:    "the team label must be platform",
			Evaluated: []LogNode{
				{Expression: "variables.team == 'platform'", Value: "false"},
				{Expression: "variables.team", Value: "payments"},
			},
		}, entry.Verdict)
	})

	t.Run("a passed decision has no breakdown", func(t *testing.T) {
		d := failedPodDecision()
		d.Verdict.Status = VerdictPass
		d.Verdict.Result = "true"
		entry, pairs := logged(t, d)
		assert.Equal(t, VerdictPass, pairs["verdict"])
		assert.Empty(t, entry.Verdict.Evaluated)
		assert.Equal(t, "variables.team == 'platform'", entry.Verdict.Expression)
	})

	t.Run("several validations are listed with their own status", func(t *testing.T) {
		d := failedPodDecision()
		d.Validations = []ValidationTrace{
			{Index: 0, Status: VerdictPass, ExpressionTrace: ExpressionTrace{Source: "has(object.metadata.labels)", Result: "true"}},
			{Index: 1, Status: VerdictFail, ExpressionTrace: ExpressionTrace{Source: "variables.team == 'platform'", Result: "false"}},
			{Index: 2, Status: VerdictNotRun, ExpressionTrace: ExpressionTrace{Source: "object.spec.replicas > 1"}},
		}
		entry, _ := logged(t, d)
		assert.Equal(t, []LogExpression{
			{Status: VerdictPass, Expression: "has(object.metadata.labels)", Result: "true"},
			{Status: VerdictFail, Expression: "variables.team == 'platform'", Result: "false"},
			{Status: VerdictNotRun, Expression: "object.spec.replicas > 1"},
		}, entry.Validations)
	})

	t.Run("a long value is cut", func(t *testing.T) {
		d := failedPodDecision()
		long := strings.Repeat("x", 3*maxValueLen)
		d.Variables[0].Result = long
		d.Verdict.Nodes[1].Value = long
		entry, _ := logged(t, d)
		assert.Equal(t, strings.Repeat("x", maxValueLen)+"...", entry.Variables[0].Result)
		assert.Equal(t, strings.Repeat("x", maxValueLen)+"...", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("a mutation that failed keeps its breakdown, one that ran does not", func(t *testing.T) {
		entry, _ := logged(t, &Decision{
			PolicyKind: "MutatingPolicy",
			Mutations: []MutationTrace{
				{Name: "mutations[0] (applyConfiguration)", ExpressionTrace: ExpressionTrace{Source: "Object{}", Result: "{}", Nodes: []NodeTrace{{Expression: "Object{}", Value: "{}"}}}},
				{Name: "mutations[1] (jsonPatch)", ExpressionTrace: ExpressionTrace{Source: "[...]", Result: "[...]"}, NotApplied: "a test operation failed"},
				{Name: "mutations[2] (applyConfiguration)", Error: "no such key: team", ExpressionTrace: ExpressionTrace{Source: "object.metadata.labels.team", Nodes: []NodeTrace{{Expression: "object.metadata.labels", Error: "no such key: team"}}}},
			},
			Verdict: VerdictTrace{Status: VerdictError, Message: "no such key: team"},
		})
		require.Len(t, entry.Mutations, 3)
		assert.Empty(t, entry.Mutations[0].Evaluated)
		assert.Equal(t, "a test operation failed", entry.Mutations[1].NotApplied)
		assert.Equal(t, "no such key: team", entry.Mutations[2].Error)
		assert.Equal(t, []LogNode{{Expression: "object.metadata.labels", Error: "no such key: team"}}, entry.Mutations[2].Evaluated)
	})
}

// TestLogValues_SecretValuesAreNotLogged: the trace of a decision on a Secret keeps every
// expression and which of them did not hold, but none of the values read from the Secret.
func TestLogValues_SecretValuesAreNotLogged(t *testing.T) {
	const password = "hunter2-do-not-log"
	d := &Decision{
		PolicyName:   "secret-rules",
		PolicyKind:   "ValidatingPolicy",
		ResourceKind: "Secret",
		ResourceName: "db-credentials",
		Match:        []NamedExpressionTrace{{Name: "has-data", ExpressionTrace: ExpressionTrace{Source: "has(object.data)", Result: "true"}}},
		Variables:    []NamedExpressionTrace{{Name: "password", ExpressionTrace: ExpressionTrace{Source: "object.data.password", Result: password}}},
		Validations: []ValidationTrace{
			{Index: 0, Status: VerdictPass, ExpressionTrace: ExpressionTrace{Source: "object.type", Result: password}},
			{Index: 1, Status: VerdictFail, ExpressionTrace: ExpressionTrace{Source: "variables.password.size() > 32", Result: "false"}},
		},
		Mutations: []MutationTrace{{Name: "mutations[0] (jsonPatch)", Error: "failed", ExpressionTrace: ExpressionTrace{
			Source: "[JSONPatch{op: 'add', path: '/data/copy', value: object.data.password}]",
			Result: password,
			Nodes:  []NodeTrace{{Expression: "object.data.password", Value: password}},
		}}},
		Verdict: VerdictTrace{
			Status: VerdictFail,
			ExpressionTrace: ExpressionTrace{
				Source: "variables.password.size() > 32",
				Result: "false",
				Nodes: []NodeTrace{
					{Expression: "variables.password.size() > 32", Value: "false"},
					{Expression: "variables.password.size()", Value: "18"},
					{Expression: "variables.password", Value: password},
				},
			},
		},
	}
	entry, _ := logged(t, d)
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), password)

	assert.Equal(t, "true", entry.Match[0].Result, "true and false say nothing about the data, and are kept")
	assert.Equal(t, redacted, entry.Variables[0].Result)
	assert.Equal(t, redacted, entry.Validations[0].Result)
	assert.Equal(t, "false", entry.Validations[1].Result)
	assert.Equal(t, redacted, entry.Mutations[0].Result)
	assert.Equal(t, redacted, entry.Mutations[0].Evaluated[0].Value)
	assert.Equal(t, "variables.password.size() > 32", entry.Verdict.Expression, "expressions are policy text, and are kept")
	assert.Equal(t, []LogNode{
		{Expression: "variables.password.size() > 32", Value: "false"},
		{Expression: "variables.password.size()", Value: redacted},
		{Expression: "variables.password", Value: redacted},
	}, entry.Verdict.Evaluated)

	// the same decision on any other kind is logged with its values
	d.ResourceKind = "ConfigMap"
	entry, _ = logged(t, d)
	assert.Equal(t, password, entry.Variables[0].Result)
}

// TestLogValues_JSON pins the field names a logged trace is written with.
func TestLogValues_JSON(t *testing.T) {
	entry, _ := logged(t, failedPodDecision())
	raw, err := json.Marshal(entry)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"scope": "matched resourceRules[0]",
		"match": [{"name": "not-system", "expression": "object.metadata.namespace != 'kube-system'", "result": "true"}],
		"variables": [{"name": "team", "expression": "object.metadata.labels.team", "result": "payments"}],
		"verdict": {
			"status": "FAIL",
			"expression": "variables.team == 'platform'",
			"result": "false",
			"message": "the team label must be platform",
			"evaluated": [
				{"expression": "variables.team == 'platform'", "value": "false"},
				{"expression": "variables.team", "value": "payments"}
			]
		}
	}`, string(raw))
}
