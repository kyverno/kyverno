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

func TestLogValues_SecretDataRedactedInErrorsAndExternalGets(t *testing.T) {
	const password = "hunter2-super-secret"

	t.Run("CEL error quoting secret value is redacted for Secret admission", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "check-secret-expiry",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Secret",
			ResourceName: "db-credentials",
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "invalid RFC 3339 timestamp \"" + password + "\"",
				ExpressionTrace: ExpressionTrace{
					Source: "timestamp(object.data.password) < timestamp('2026-01-01T00:00:00Z')",
					Result: "invalid RFC 3339 timestamp \"" + password + "\"",
					Nodes: []NodeTrace{
						{
							Expression: "timestamp(object.data.password)",
							Error:      "invalid RFC 3339 timestamp \"" + password + "\"",
						},
						{
							Expression: "object.data.password",
							Value:      password,
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Result)
		assert.Equal(t, redacted, entry.Verdict.Message)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Error)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Secret read by policy on Pod via resource.Get or variable is redacted", func(t *testing.T) {
		secretObjectStr := `map[apiVersion:v1 data:map[password:` + password + `] kind:Secret metadata:map[name:db-credentials namespace:default]]`
		d := &Decision{
			PolicyName:   "validate-pod-secret",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db-credentials')",
						Result: secretObjectStr,
					},
				},
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.dbSecret.data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.dbPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "variables.dbPass == 'expected'",
							Value:      "false",
						},
						{
							Expression: "variables.dbPass",
							Value:      password,
						},
						{
							Expression: "variables.dbSecret.data",
							Value:      "map[password:" + password + "]",
						},
						{
							Expression: "variables.dbSecret",
							Value:      secretObjectStr,
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)

		require.Len(t, entry.Variables, 2)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		assert.Equal(t, redacted, entry.Variables[1].Result)

		require.Len(t, entry.Verdict.Evaluated, 4)
		assert.Equal(t, "false", entry.Verdict.Evaluated[0].Value, "boolean verdict value is kept")
		assert.Equal(t, redacted, entry.Verdict.Evaluated[1].Value)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[2].Value)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[3].Value)
	})

	t.Run("CEL error on secret variable in Pod policy is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "pod-secret-error",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db-credentials')",
						Result: `map[apiVersion:v1 data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "invalid RFC 3339 timestamp \"" + password + "\"",
				ExpressionTrace: ExpressionTrace{
					Source: "timestamp(variables.dbSecret.data.password) < timestamp('2026-01-01T00:00:00Z')",
					Result: "invalid RFC 3339 timestamp \"" + password + "\"",
					Nodes: []NodeTrace{
						{
							Expression: "timestamp(variables.dbSecret.data.password)",
							Error:      "invalid RFC 3339 timestamp \"" + password + "\"",
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Result)
		assert.Equal(t, redacted, entry.Verdict.Message)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Error)
	})

	t.Run("Variable reference patterns (optional traversal, prefix collisions, index access) are redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "var-patterns",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "sec",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 's')",
						Result: `map[kind:Secret]`,
					},
				},
				{
					Name: "sec_backup",
					ExpressionTrace: ExpressionTrace{
						Source: "object.metadata.name",
						Result: "my-pod",
					},
				},
				{
					Name: "derived",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.sec_backup == 'my-pod' ? variables.?sec.data.pass : 'fallback'",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.sec_backup == 'my-pod' && variables['sec'].data.pass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables['sec'].data.pass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		assert.Equal(t, "my-pod", entry.Variables[1].Result, "non-secret variable is kept")
		assert.Equal(t, redacted, entry.Variables[2].Result, "derived secret variable is redacted")
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Direct resource.Get in validation without variable is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "direct-get",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get('v1', 'secrets', 'default', 's').data.pass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "resource.Get('v1', 'secrets', 'default', 's').data.pass == 'expected'",
							Value:      "false",
						},
						{
							Expression: "resource.Get('v1', 'secrets', 'default', 's').data.pass",
							Value:      password,
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "false", entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Mutating policy error and value reading Secret are redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "mutate-with-secret",
			PolicyKind:   "MutatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[token:` + password + `] kind:Secret]`,
					},
				},
			},
			Mutations: []MutationTrace{
				{
					Name:  "mutations[0] (apply)",
					Error: "failed applying token \"" + password + "\"",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.dbSecret.data.token",
						Result: password,
						Nodes: []NodeTrace{
							{
								Expression: "variables.dbSecret.data.token",
								Error:      "failed to parse token \"" + password + "\"",
							},
						},
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "failed applying token \"" + password + "\"",
				ExpressionTrace: ExpressionTrace{
					Source: "variables.dbSecret.data.token",
					Result: "failed applying token \"" + password + "\"",
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Result)
		assert.Equal(t, redacted, entry.Verdict.Message)
		require.Len(t, entry.Mutations, 1)
		assert.Equal(t, redacted, entry.Mutations[0].Result)
		assert.Equal(t, redacted, entry.Mutations[0].Error)
		require.Len(t, entry.Mutations[0].Evaluated, 1)
		assert.Equal(t, redacted, entry.Mutations[0].Evaluated[0].Error)
	})

	t.Run("Mutating policy with source-less verdict redacts message quoting secret", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "mutate-sourceless",
			PolicyKind:   "MutatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[token:` + password + `] kind:Secret]`,
					},
				},
			},
			Mutations: []MutationTrace{
				{
					Name:  "mutations[0] (apply)",
					Error: "failed applying token \"" + password + "\"",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.dbSecret.data.token",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "failed applying token \"" + password + "\"",
				// source-less verdict as generated by mpol
				ExpressionTrace: ExpressionTrace{
					Source: "",
					Result: "",
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		require.Len(t, entry.Mutations, 1)
		assert.Equal(t, redacted, entry.Mutations[0].Error)
	})

	t.Run("Secret read via indirect GVR variable is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "indirect-gvr",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "secGvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'Secret')",
						Result: "/v1, Resource=secrets",
					},
				},
				{
					Name: "password",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get(variables.secGvr, 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.password == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.password", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("ConfigMap read comparing to literal 'secret' is not falsely redacted", func(t *testing.T) {
		const cmVal = "public-config"
		d := &Decision{
			PolicyName:   "cm-check",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "cmData",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'configmaps', 'default', 'app').data.val",
						Result: cmVal,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get('v1', 'configmaps', 'default', 'app').data.mode == 'secret'",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "variables.cmData",
							Value:      cmVal,
						},
					},
				},
			},
		}

		assert.False(t, newLogFormat(d).isSecretExpr(d.Verdict.Source), "ConfigMap read comparing to literal 'secret' must not be classified as secret")
		entry, _ := logged(t, d)
		assert.Equal(t, cmVal, entry.Variables[0].Result, "ConfigMap value must not be redacted")
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, cmVal, entry.Verdict.Evaluated[0].Value, "ConfigMap node value must not be redacted")
	})

	t.Run("Consumer-first variable ordering redacts secret data via fixed-point propagation", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "consumer-first",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				// Consumer listed first before producer
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.dbSecret.data.password",
						Result: password,
					},
				},
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.dbPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.dbPass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Long non-secret errors in verdict and nodes are clipped", func(t *testing.T) {
		longError := "non-secret failure: " + strings.Repeat("e", 3*maxValueLen)
		d := &Decision{
			PolicyName:   "long-error",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: longError,
				ExpressionTrace: ExpressionTrace{
					Source: "object.spec.replicas > 0",
					Result: longError,
					Nodes: []NodeTrace{
						{Expression: "object.spec.replicas", Error: longError},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		assert.Equal(t, clip(longError), entry.Verdict.Result)
		assert.Equal(t, clip(longError), entry.Verdict.Message)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, clip(longError), entry.Verdict.Evaluated[0].Error)
	})

	t.Run("Secret read via GVR variable alias chain is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "gvr-alias-chain",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "gvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'Secret')",
						Result: "/v1, Resource=secrets",
					},
				},
				{
					Name: "alias",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.gvr",
						Result: "/v1, Resource=secrets",
					},
				},
				{
					Name: "sec",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get(variables.alias, 'default', 'db')",
						Result: `map[data:map[password:` + password + `]]`,
					},
				},
				{
					Name: "pass",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.sec.data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.pass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.pass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[2].Result)
		assert.Equal(t, redacted, entry.Variables[3].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Secret read via GVR variable alias with unclassifiable result is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "gvr-alias-source-only",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "gvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'Secret')",
						Result: "/v1, Resource=secrets",
					},
				},
				{
					Name: "alias",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.gvr",
						Result: "<gvr-alias-opaque>",
					},
				},
				{
					Name: "sec",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get(variables.alias, 'default', 'db')",
						Result: `map[data:map[password:` + password + `]]`,
					},
				},
				{
					Name: "pass",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.sec.data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.pass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.pass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[2].Result)
		assert.Equal(t, redacted, entry.Variables[3].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Secret read with variable resource name in resource.Get is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "variable-resource-name",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "resourceKind",
					ExpressionTrace: ExpressionTrace{
						Source: "'secrets'",
						Result: "secrets",
					},
				},
				{
					Name: "sec",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', variables.resourceKind, 'default', 'db')",
						Result: `map[data:map[password:` + password + `]]`,
					},
				},
				{
					Name: "pass",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.sec.data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.pass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.pass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, "secrets", entry.Variables[0].Result, "Resource name variable itself should not be redacted")
		assert.Equal(t, redacted, entry.Variables[1].Result)
		assert.Equal(t, redacted, entry.Variables[2].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Source-less verdict redacts error from failing match condition reading secret", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "mpol-match-error",
			PolicyKind:   "MutatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Match: []NamedExpressionTrace{
				{
					Name: "check-pass",
					ExpressionTrace: ExpressionTrace{
						Source: "timestamp(variables.dbSecret.data.password)",
						Result: "invalid RFC 3339 timestamp \"" + password + "\"",
					},
				},
			},
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "invalid RFC 3339 timestamp \"" + password + "\"",
				// Source is empty for source-less verdict
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, redacted, entry.Match[0].Result)
		assert.Equal(t, redacted, entry.Variables[0].Result)
	})

	t.Run("Dynamic indexing through dyn(variables) is redacted when secret variables present", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "dynamic-variables-access",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "dyn(variables)[object.metadata.labels.key] == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "dyn(variables)[object.metadata.labels.key]", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Sensitivity propagates to subsequent mutations in mutating policy", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "mpol-sequential-mutations",
			PolicyKind:   "MutatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "sec",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Mutations: []MutationTrace{
				{
					Name: "mutations[0] (copySecret)",
					ExpressionTrace: ExpressionTrace{
						Source: "variables.sec.data.password",
						Result: password,
					},
				},
				{
					Name: "mutations[1] (readModifiedObject)",
					Error: "failed patch with value \"" + password + "\"",
					ExpressionTrace: ExpressionTrace{
						Source: "object.spec.containers[0].env[0].value",
						Result: password,
						Nodes: []NodeTrace{
							{Expression: "object.spec.containers[0].env[0].value", Value: password},
						},
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "failed patch with value \"" + password + "\"",
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, redacted, entry.Mutations[0].Result)
		assert.Equal(t, redacted, entry.Mutations[1].Result)
		assert.Equal(t, redacted, entry.Mutations[1].Error)
		require.Len(t, entry.Mutations[1].Evaluated, 1)
		assert.Equal(t, redacted, entry.Mutations[1].Evaluated[0].Value)
	})

	t.Run("Secret read via globalContext.get is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "gctx-secret",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "projSecret",
					ExpressionTrace: ExpressionTrace{
						Source: `globalContext.get("secrets-cache", "")`,
						Result: `map[password:` + password + `]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.projSecret.password == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.projSecret.password", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Generating policy with source-less verdict redacts message quoting secret from generation rule", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "gpol-secret-gen",
			PolicyKind:   "GeneratingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Generations: []GenerationTrace{
				{
					Name: "generate[0] (expression)",
					ExpressionTrace: ExpressionTrace{
						Source: "generator.Apply('default', [variables.dbSecret])",
						Result: "false",
					},
					Error: "failed applying secret \"" + password + "\"",
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: "failed applying secret \"" + password + "\"",
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, redacted, entry.Variables[0].Result)
	})

	t.Run("Secret variable read through variables alias is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "variables-alias",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "vars",
					ExpressionTrace: ExpressionTrace{
						Source: "variables",
						Result: "<map>",
					},
				},
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "vars.dbSecret.data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "vars.dbPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "vars.dbPass", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		assert.Equal(t, redacted, entry.Variables[2].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Secret read via globalContext.get with arbitrary entry name is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "gctx-arbitrary-name",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "cachedAuth",
					ExpressionTrace: ExpressionTrace{
						Source: `globalContext.get("external-auth-cache", "")`,
						Result: `map[token:` + password + `]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.cachedAuth.token == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.cachedAuth.token", Value: password},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		require.Len(t, entry.Verdict.Evaluated, 1)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
	})

	t.Run("VerdictFail with secret message from messageExpression is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "vpol-msg-expr-secret",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Secret",
			ResourceName: "my-secret",
			Verdict: VerdictTrace{
				Status:  VerdictFail,
				Message: "Password validation failed: " + password,
				ExpressionTrace: ExpressionTrace{
					Source: "false",
					Result: "false",
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, "false", entry.Verdict.Result)
	})

	t.Run("Secret variable taints verdict message on non-secret validation source", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "vpol-var-taint-msg",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'Secret', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictFail,
				Message: "Authentication failed with secret token: " + password,
				ExpressionTrace: ExpressionTrace{
					Source: "object.spec.replicas > 1",
					Result: "false",
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, "false", entry.Verdict.Result)
	})

	t.Run("Template-based generation placeholder error reading secret is redacted", func(t *testing.T) {
		placeholderErr := `failed to render generation template: document 1: field "data": field "password": failed to evaluate placeholder "resource.Get('v1', 'Secret', 'default', 'db').data.password": connection refused for ` + password
		d := &Decision{
			PolicyName:   "gpol-template-placeholder",
			PolicyKind:   "GeneratingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Generations: []GenerationTrace{
				{
					Name: "generate[0] (template)",
					ExpressionTrace: ExpressionTrace{
						Source: "",
					},
					Error: placeholderErr,
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictError,
				Message: placeholderErr,
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
	})

	t.Run("Source-less verdict with scalar secret variable is redacted on FAIL", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "sourceless-scalar-secret-var",
			PolicyKind:   "GeneratingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'Secret', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status:  VerdictFail,
				Message: "Generation precondition failed quoting " + password,
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
	})

	t.Run("Trace breakdown node values in secret expressions are redacted while preserving boolean results", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "breakdown-node-values",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get('v1', 'Secret', 'default', 'db').data.token != ''",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "resource.Get('v1', 'Secret', 'default', 'db').data.token", Value: password},
						{Expression: "resource.Get('v1', 'Secret', 'default', 'db').data.token != ''", Value: "false"},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Dynamic resource selector in resource.Get is conservatively redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "dynamic-resource-selector",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get('v1', object.metadata.labels.kind == 'secret' ? 'secrets' : 'configmaps', 'default', 'db').data.password != ''",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "resource.Get('v1', object.metadata.labels.kind == 'secret' ? 'secrets' : 'configmaps', 'default', 'db').data.password",
							Value:      password,
						},
						{
							Expression: "resource.Get('v1', object.metadata.labels.kind == 'secret' ? 'secrets' : 'configmaps', 'default', 'db').data.password != ''",
							Value:      "false",
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Literal indexing through dyn(variables) is recognized and redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "dyn-variables-literal-indexing",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: `dyn(variables)["dbSecret"].data.password == 'expected'`,
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: `dyn(variables)["dbSecret"].data.password`, Value: password},
						{Expression: `dyn(variables)["dbSecret"].data.password == 'expected'`, Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr(`dyn ( variables ) [ "dbSecret" ].data.password`))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Variables map root and aliases are redacted when secretVars is non-empty", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "variables-root-redacted",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
				{
					Name: "allVars",
					ExpressionTrace: ExpressionTrace{
						Source: "variables",
						Result: `map[dbSecret:map[password:` + password + `]]`,
					},
				},
				{
					Name: "dynVars",
					ExpressionTrace: ExpressionTrace{
						Source: "dyn(variables)",
						Result: `map[dbSecret:map[password:` + password + `]]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.allVars != null",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables", Value: `map[dbSecret:map[password:` + password + `]]`},
						{Expression: "variables.allVars != null", Value: "false"},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		assert.Equal(t, redacted, entry.Variables[2].Result)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Unrelated CRDs starting with Secret (e.g. SecretStore) are not over-redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "secret-store-crd-not-redacted",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "storeManifest",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('external-secrets.io/v1beta1', 'secretstores', 'default', 'my-store')",
						Result: `map[apiVersion:external-secrets.io/v1beta1 kind:SecretStore metadata:map[name:my-store]]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictPass,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.storeManifest.metadata.name == 'my-store'",
					Result: "true",
					Nodes: []NodeTrace{
						{Expression: "variables.storeManifest.metadata.name", Value: "my-store"},
						{Expression: "variables.storeManifest.metadata.name == 'my-store'", Value: "true"},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "my-store")
		assert.Contains(t, string(raw), "SecretStore")
		assert.NotEqual(t, redacted, entry.Variables[0].Result)
	})

	t.Run("Direct literal non-Secret ToGVR call in resource.Get is not redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "direct-non-secret-togvr",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get(resource.ToGVR('v1', 'ConfigMap'), 'default', 'cm').data.key == 'val'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "resource.Get(resource.ToGVR('v1', 'ConfigMap'), 'default', 'cm').data.key", Value: "val"},
						{Expression: "resource.Get(resource.ToGVR('v1', 'ConfigMap'), 'default', 'cm').data.key == 'val'", Value: "false"},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "val")
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "val", entry.Verdict.Evaluated[0].Value)
	})

	t.Run("Case-sensitive variables alias (e.g. Vars) is recognized and redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "case-sensitive-alias",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "Vars",
					ExpressionTrace: ExpressionTrace{
						Source: "variables",
						Result: "<map>",
					},
				},
				{
					Name: "dbSecret",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db')",
						Result: `map[data:map[password:` + password + `] kind:Secret]`,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: `dyn(Vars)["dbSecret"].data.password == 'expected'`,
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: `dyn(Vars)["dbSecret"].data.password`, Value: password},
						{Expression: `dyn(Vars)["dbSecret"].data.password == 'expected'`, Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr(`dyn ( Vars ) [ object.metadata.labels.key ]`))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Taint variables accessed through collections (e.g. [dyn(variables)][0].dbPass)", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "collection-variable-access",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
				{
					Name: "collectedPass",
					ExpressionTrace: ExpressionTrace{
						Source: "[dyn(variables)][0].dbPass",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.collectedPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.collectedPass", Value: password},
						{Expression: "variables.collectedPass == 'expected'", Value: "false"},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[0].Result)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Conditional resource.Get selector with safe and secret GVR is tainted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "conditional-safe-and-secret-gvr",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "cmGvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'ConfigMap')",
						Result: "v1/ConfigMap",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get(object.spec.useSecret ? resource.ToGVR('v1', 'Secret') : variables.cmGvr, 'default', 'db').data.password != ''",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "resource.Get(object.spec.useSecret ? resource.ToGVR('v1', 'Secret') : variables.cmGvr, 'default', 'db').data.password",
							Value:      password,
						},
						{
							Expression: "resource.Get(object.spec.useSecret ? resource.ToGVR('v1', 'Secret') : variables.cmGvr, 'default', 'db').data.password != ''",
							Value:      "false",
						},
					},
				},
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Secret variable referenced through comprehension binding (e.g. [variables].all(v, v.dbPass == ...)) is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "comprehension-variable-access",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[variables].all(v, v.dbPass == 'expected')",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "v.dbPass", Value: password},
						{Expression: "[variables].all(v, v.dbPass == 'expected')", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[variables].all(v, v.dbPass == 'expected')"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Safe GVR variable name shadowed by comprehension iterator is not exempted as safe", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "shadowed-safe-gvr",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "cmGvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'ConfigMap')",
						Result: "v1/ConfigMap",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[resource.ToGVR('v1', 'Secret')].all(cmGvr, resource.Get(cmGvr, 'default', 'db').data.password != '')",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "resource.Get(cmGvr, 'default', 'db').data.password",
							Value:      password,
						},
						{
							Expression: "[resource.ToGVR('v1', 'Secret')].all(cmGvr, resource.Get(cmGvr, 'default', 'db').data.password != '')",
							Value:      "false",
						},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[resource.ToGVR('v1', 'Secret')].all(cmGvr, resource.Get(cmGvr, 'default', 'db').data.password != '')"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)

		// Safe-GVR control: unshadowed cmGvr reading ConfigMap is NOT redacted
		dSafe := &Decision{
			PolicyName:   "unshadowed-safe-gvr",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "cmGvr",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.ToGVR('v1', 'ConfigMap')",
						Result: "v1/ConfigMap",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Get(variables.cmGvr, 'default', 'cm').data.config == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "resource.Get(variables.cmGvr, 'default', 'cm').data.config",
							Value:      "safe-config",
						},
						{
							Expression: "resource.Get(variables.cmGvr, 'default', 'cm').data.config == 'expected'",
							Value:      "false",
						},
					},
				},
			},
		}
		assert.False(t, newLogFormat(dSafe).isSecretExpr("resource.Get(variables.cmGvr, 'default', 'cm').data.config == 'expected'"))
		entrySafe, _ := logged(t, dSafe)
		rawSafe, errSafe := json.Marshal(entrySafe)
		require.NoError(t, errSafe)
		assert.Contains(t, string(rawSafe), "safe-config")
		require.Len(t, entrySafe.Verdict.Evaluated, 2)
		assert.Equal(t, "safe-config", entrySafe.Verdict.Evaluated[0].Value)
	})

	t.Run("Collection-derived variable root [dyn(variables)][0] is marked sensitive when secret variables present", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "collection-derived-root",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
				{
					Name: "rootVars",
					ExpressionTrace: ExpressionTrace{
						Source: "[dyn(variables)][0]",
						Result: "<map>",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "rootVars[object.metadata.labels.key] != ''",
					Result: "false",
					Nodes: []NodeTrace{
						{
							Expression: "rootVars[object.metadata.labels.key]",
							Value:      password,
						},
						{
							Expression: "rootVars[object.metadata.labels.key] != ''",
							Value:      "false",
						},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[dyn(variables)][0][object.metadata.labels.key]"))
		assert.True(t, newLogFormat(d).isSecretExpr("rootVars[object.metadata.labels.key]"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Variables[1].Result)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Comprehension binding wrapped by dyn and carried through collection [variables].map(v, dyn(v).dbPass)[0] is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "nested-dyn-comprehension",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[variables].map(v, dyn(v).dbPass)[0] == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "[variables].map(v, dyn(v).dbPass)[0]", Value: password},
						{Expression: "[variables].map(v, dyn(v).dbPass)[0] == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[variables].map(v, dyn(v).dbPass)[0] == 'expected'"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Optional field-selection on comprehension binding v.?dbPass.orValue(...) is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "optional-field-comprehension",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[variables].all(v, v.?dbPass.orValue('default') == 'expected')",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "v.?dbPass.orValue('default')", Value: password},
						{Expression: "[variables].all(v, v.?dbPass.orValue('default') == 'expected')", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[variables].all(v, v.?dbPass.orValue('default') == 'expected')"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Scalar indexing [variables.team][0] is not treated as a variables-root alias and retains public value", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "scalar-index-not-root",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
				{
					Name: "team",
					ExpressionTrace: ExpressionTrace{
						Source: "'platform'",
						Result: "platform",
					},
				},
				{
					Name: "scalarCopy",
					ExpressionTrace: ExpressionTrace{
						Source: "[variables.team][0]",
						Result: "platform",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "variables.scalarCopy == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "variables.scalarCopy", Value: "platform"},
						{Expression: "variables.scalarCopy == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.False(t, newLogFormat(d).isSecretExpr("variables.scalarCopy == 'expected'"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "platform")
		assert.Equal(t, "platform", entry.Variables[2].Result)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "platform", entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Literal index dyn(variables)['team'] is not treated as dynamic access when secret variable present", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "literal-dyn-index-not-dynamic",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
				{
					Name: "team",
					ExpressionTrace: ExpressionTrace{
						Source: "'platform'",
						Result: "platform",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "dyn(variables)['team'] == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "dyn(variables)['team']", Value: "platform"},
						{Expression: "dyn(variables)['team'] == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.False(t, newLogFormat(d).isSecretExpr("dyn(variables)['team'] == 'expected'"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "platform")
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "platform", entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Secret-targeting resource.Post result and scalar projection are redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "post-secret-policy",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "resource.Post('v1', 'secrets', 'default', request.payload).data.password == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "resource.Post('v1', 'secrets', 'default', request.payload).data.password", Value: password},
						{Expression: "resource.Post('v1', 'secrets', 'default', request.payload).data.password == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).readsSecret("resource.Post('v1', 'secrets', 'default', request.payload).data.password"))
		assert.True(t, newLogFormat(d).isSecretExpr("resource.Post('v1', 'secrets', 'default', request.payload).data.password"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Secret value in validation messageExpression is redacted for non-secret admission", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "vpol-msg-expr-secret",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "false",
					Result: "false",
				},
				Message:           "secret database password is " + password,
				MessageExpression: "resource.Get('v1', 'secrets', 'default', 'db-secret').data.password",
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, redacted, entry.Verdict.Message)
		assert.Equal(t, "false", entry.Verdict.Result)
	})

	t.Run("Error trace preserves boolean results for Secret decisions", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "secret-error-bool-preserved",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Secret",
			ResourceName: "my-secret",
			Verdict: VerdictTrace{
				Status: VerdictError,
				ExpressionTrace: ExpressionTrace{
					Source: "object.data.password.size() > 0",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "object.data.password.size() > 0", Value: "false"},
						{Expression: "object.data.password.size()", Error: "type error: " + password},
					},
				},
				Message: "evaluation error: " + password,
			},
		}

		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		assert.Equal(t, "false", entry.Verdict.Result)
		assert.Equal(t, redacted, entry.Verdict.Message)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "false", entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[1].Error)
	})

	t.Run("Nested collection [[variables]][0][0].dbPass propagates provenance and redacts secret", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "nested-collection-var-root",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[[variables]][0][0].dbPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "[[variables]][0][0].dbPass", Value: password},
						{Expression: "[[variables]][0][0].dbPass == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[[variables]][0][0].dbPass == 'expected'"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Mapped collection [variables].map(v, v)[0].dbPass propagates provenance and redacts secret", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "mapped-collection-var-root",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: "[variables].map(v, v)[0].dbPass == 'expected'",
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: "[variables].map(v, v)[0].dbPass", Value: password},
						{Expression: "[variables].map(v, v)[0].dbPass == 'expected'", Value: "false"},
					},
				},
			},
		}

		assert.True(t, newLogFormat(d).isSecretExpr("[variables].map(v, v)[0].dbPass == 'expected'"))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.NotContains(t, string(raw), password)
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, redacted, entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})

	t.Run("Double-quoted literal index dyn(variables)[\"team\"] preserves value while mixed dynamic index is redacted", func(t *testing.T) {
		d := &Decision{
			PolicyName:   "double-quoted-literal-dyn-index",
			PolicyKind:   "ValidatingPolicy",
			ResourceKind: "Pod",
			ResourceName: "my-pod",
			Variables: []NamedExpressionTrace{
				{
					Name: "dbPass",
					ExpressionTrace: ExpressionTrace{
						Source: "resource.Get('v1', 'secrets', 'default', 'db').data.password",
						Result: password,
					},
				},
				{
					Name: "team",
					ExpressionTrace: ExpressionTrace{
						Source: "'platform'",
						Result: "platform",
					},
				},
			},
			Verdict: VerdictTrace{
				Status: VerdictFail,
				ExpressionTrace: ExpressionTrace{
					Source: `dyn(variables)["team"] == 'expected'`,
					Result: "false",
					Nodes: []NodeTrace{
						{Expression: `dyn(variables)["team"]`, Value: "platform"},
						{Expression: `dyn(variables)["team"] == 'expected'`, Value: "false"},
					},
				},
			},
		}

		assert.False(t, newLogFormat(d).isSecretExpr(`dyn(variables)["team"] == 'expected'`))
		assert.True(t, newLogFormat(d).isSecretExpr(`dyn(variables)["team"] == 'expected' && dyn(variables)[key] == 'foo'`))
		entry, _ := logged(t, d)
		raw, err := json.Marshal(entry)
		require.NoError(t, err)
		assert.Contains(t, string(raw), "platform")
		require.Len(t, entry.Verdict.Evaluated, 2)
		assert.Equal(t, "platform", entry.Verdict.Evaluated[0].Value)
		assert.Equal(t, "false", entry.Verdict.Evaluated[1].Value)
	})
}
