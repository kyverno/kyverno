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
}
