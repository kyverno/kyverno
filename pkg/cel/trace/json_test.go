package trace

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fullDecision has every part a decision can have, from every policy type at once, so one
// encoding shows every field name.
func fullDecision() *Decision {
	d := failedPodDecision()
	d.ResourceNamespace = "prod"
	d.Validations = []ValidationTrace{
		{Index: 0, Status: VerdictPass, ExpressionTrace: ExpressionTrace{Source: "has(object.metadata.labels)", Result: "true"}},
		{Index: 1, Status: VerdictFail, ExpressionTrace: ExpressionTrace{Source: "variables.team == 'platform'", Result: "false"}},
		{Index: 2, Status: VerdictNotRun, ExpressionTrace: ExpressionTrace{Source: "object.metadata.namespace == 'prod'"}},
	}
	d.Images = &ImagesTrace{Found: []ImageTrace{{Category: "containers", Image: "nginx:1.27", Checked: false}}}
	d.Verdict.LoopValuesOmitted = true
	d.Verdict.NoBreakdown = "it calls random, which is not run a second time"
	d.Mutations = []MutationTrace{{
		Name:            "mutations[0] (applyConfiguration)",
		ExpressionTrace: ExpressionTrace{Source: "Object{metadata: Object.metadata{labels: {'team': 'platform'}}}", Result: "map[metadata:map[labels:map[team:platform]]]"},
		Error:           "failed to apply",
		NotApplied:      "it changed nothing",
	}}
	d.Generations = []GenerationTrace{{
		Name:            "generate[0] (expression)",
		ExpressionTrace: ExpressionTrace{Source: "generator.Apply('prod', [variables.cm])", Result: "true"},
		Generated:       []string{"ConfigMap prod/team-config"},
		Error:           "failed to generate",
	}}
	return d
}

// TestDecision_JSON pins the field names a decision is written with.
func TestDecision_JSON(t *testing.T) {
	raw, err := json.Marshal(fullDecision())
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"policy": {"name": "require-team-label", "kind": "ValidatingPolicy"},
		"resource": {"kind": "Pod", "name": "web", "namespace": "prod"},
		"scope": {"applied": true, "reason": "matched resourceRules[0]"},
		"match": [{"name": "not-system", "expression": "object.metadata.namespace != 'kube-system'", "result": "true"}],
		"variables": [{"name": "team", "expression": "object.metadata.labels.team", "result": "payments"}],
		"validations": [
			{"index": 0, "status": "PASS", "expression": "has(object.metadata.labels)", "result": "true"},
			{"index": 1, "status": "FAIL", "expression": "variables.team == 'platform'", "result": "false"},
			{"index": 2, "status": "NOT RUN", "expression": "object.metadata.namespace == 'prod'"}
		],
		"images": {"found": [{"category": "containers", "image": "nginx:1.27", "checked": false}]},
		"verdict": {
			"status": "FAIL",
			"expression": "variables.team == 'platform'",
			"result": "false",
			"message": "the team label must be platform",
			"evaluated": [
				{"expression": "variables.team == 'platform'", "value": "false"},
				{"expression": "variables.team", "value": "payments"}
			],
			"loopValuesOmitted": true,
			"noBreakdown": "it calls random, which is not run a second time"
		},
		"mutations": [{
			"name": "mutations[0] (applyConfiguration)",
			"expression": "Object{metadata: Object.metadata{labels: {'team': 'platform'}}}",
			"result": "map[metadata:map[labels:map[team:platform]]]",
			"error": "failed to apply",
			"notApplied": "it changed nothing"
		}],
		"generations": [{
			"name": "generate[0] (expression)",
			"expression": "generator.Apply('prod', [variables.cm])",
			"result": "true",
			"generated": ["ConfigMap prod/team-config"],
			"error": "failed to generate"
		}]
	}`, string(raw))
}

// TestDecision_JSONRoundTrip: a decision reads back as the decision that was written.
func TestDecision_JSONRoundTrip(t *testing.T) {
	for name, d := range map[string]*Decision{
		"every part":     fullDecision(),
		"a JSON payload": {PolicyName: "check-payload", PolicyKind: "ValidatingPolicy", Scope: ScopeTrace{Applied: true, Reason: "evaluated against a JSON payload, so no matchConstraints apply"}, Verdict: VerdictTrace{Status: VerdictPass}},
		"nothing found":  {PolicyName: "check-images", Images: &ImagesTrace{}, Verdict: VerdictTrace{Status: VerdictPass}},
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(d)
			require.NoError(t, err)
			var read Decision
			require.NoError(t, json.Unmarshal(raw, &read))
			assert.Equal(t, d, &read)
		})
	}
}

// TestDecision_JSONHidesSecretData: encoding a decision hides what Render and LogValues hide, and
// leaves the decision itself as it was.
func TestDecision_JSONHidesSecretData(t *testing.T) {
	d := secretDecision(t)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "czNjcjN0LXA0c3M=", "the Secret's data")
	assert.NotContains(t, string(raw), "s3cr3t-p4ss", "the Secret's data, decoded")
	assert.Contains(t, string(raw), "strict", "a ConfigMap's value is not hidden")

	var read Decision
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, redacted, read.Variables[1].Result)
	assert.Equal(t, "czNjcjN0LXA0c3M=", d.Variables[1].Result, "the decision itself is not changed")
}

// TestDecision_JSONHidesSecretErrors: an error can quote the value it failed on, so encoding a
// decision hides the error of each expression that may read a Secret, wherever the error is kept,
// and keeps an error that has nothing to do with one. The decision itself is not changed.
func TestDecision_JSONHidesSecretErrors(t *testing.T) {
	const quoted = `invalid RFC 3339 timestamp "czNjcjN0LXA0c3M="`
	tests := []struct {
		name string
		// decision is a decision with quoted as the error of an expression with these inputs
		decision func(in Inputs) *Decision
		// written returns the error as d holds it
		written func(d *Decision) string
	}{{
		name: "a node of the breakdown",
		decision: func(in Inputs) *Decision {
			return &Decision{PolicyName: "p", Verdict: VerdictTrace{Status: VerdictFail, ExpressionTrace: ExpressionTrace{
				Source: "timestamp(variables.password) < timestamp('2026-01-01T00:00:00Z')",
				Nodes:  []NodeTrace{{Expression: "timestamp(variables.password)", Error: quoted, Inputs: in}},
			}}}
		},
		written: func(d *Decision) string { return d.Verdict.Nodes[0].Error },
	}, {
		name: "the message of an ERROR verdict",
		decision: func(in Inputs) *Decision {
			return &Decision{PolicyName: "p", Verdict: VerdictTrace{Status: VerdictError, Message: quoted, ExpressionTrace: ExpressionTrace{
				Source: "timestamp(variables.password) < timestamp('2026-01-01T00:00:00Z')",
				Inputs: in,
			}}}
		},
		written: func(d *Decision) string { return d.Verdict.Message },
	}, {
		name: "a mutation",
		decision: func(in Inputs) *Decision {
			return &Decision{PolicyName: "p", Verdict: VerdictTrace{Status: VerdictError}, Mutations: []MutationTrace{{
				Name:            "mutations[0] (jsonPatch)",
				ExpressionTrace: ExpressionTrace{Source: "[JSONPatch{op: 'add', path: '/x', value: timestamp(variables.password)}]", Inputs: in},
				Error:           quoted,
			}}}
		},
		written: func(d *Decision) string { return d.Mutations[0].Error },
	}, {
		name: "a generate entry",
		decision: func(in Inputs) *Decision {
			return &Decision{PolicyName: "p", Verdict: VerdictTrace{Status: VerdictError}, Generations: []GenerationTrace{{
				Name:            "generate[0] (expression)",
				ExpressionTrace: ExpressionTrace{Source: "generator.Apply('prod', [variables.copy])", Inputs: in},
				Error:           quoted,
			}}}
		},
		written: func(d *Decision) string { return d.Generations[0].Error },
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.decision(Inputs{Secret: true})
			raw, err := json.Marshal(d)
			require.NoError(t, err)
			assert.NotContains(t, string(raw), "czNjcjN0LXA0c3M=")
			var read Decision
			require.NoError(t, json.Unmarshal(raw, &read))
			assert.Equal(t, redacted, tt.written(&read))
			assert.Equal(t, quoted, tt.written(d), "the decision itself is not changed")

			// the same error of an expression that reads no Secret is kept
			raw, err = json.Marshal(tt.decision(Inputs{}))
			require.NoError(t, err)
			read = Decision{}
			require.NoError(t, json.Unmarshal(raw, &read))
			assert.Equal(t, quoted, tt.written(&read))
		})
	}
}

func TestDecision_JSONHidesScopeAndTemplateErrors(t *testing.T) {
	const secretVal = "czNjcjN0LXA0c3M="
	// Scope error on Secret resource
	dScope := &Decision{
		PolicyName:   "p",
		ResourceKind: "Secret",
		Scope:        ScopeTrace{Reason: "failed to evaluate matchConstraints: " + secretVal},
	}
	raw, err := json.Marshal(dScope)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), secretVal)
	var read Decision
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, redacted, read.Scope.Reason)

	// Template generation error on Pod resource
	dTemplate := &Decision{
		PolicyName:   "p",
		ResourceKind: "Pod",
		Generations: []GenerationTrace{{
			Name:  "generate[0] (template)",
			Error: "failed rendering: " + secretVal,
		}},
	}
	raw, err = json.Marshal(dTemplate)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), secretVal)
	read = Decision{}
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, redacted, read.Generations[0].Error)
}

// TestDecisionList_JSON pins a list's envelope, and that it reads back.
func TestDecisionList_JSON(t *testing.T) {
	raw, err := json.Marshal(NewDecisionList(nil, &Summary{}, nil))
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"apiVersion": "trace.kyverno.io/v1alpha1",
		"kind": "DecisionList",
		"items": [],
		"summary": {"pass": 0, "fail": 0, "warn": 0, "error": 0, "skip": 0}
	}`, string(raw), "no decisions is an empty list, not null")

	// a run that stopped before it made any decision has no summary, only why it stopped
	raw, err = json.Marshal(NewDecisionList(nil, nil, errors.New("failed to load policies")))
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"apiVersion": "trace.kyverno.io/v1alpha1",
		"kind": "DecisionList",
		"items": [],
		"error": "failed to load policies"
	}`, string(raw))

	list := NewDecisionList([]*Decision{fullDecision(), failedPodDecision()}, &Summary{Pass: 1, Fail: 1}, errors.New("failed to apply deleting policies"))
	raw, err = json.Marshal(list)
	require.NoError(t, err)
	var read DecisionList
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, list, read)
}

func TestDecisionList_JSONHidesSecretErrors(t *testing.T) {
	const secretErr = `failed to apply policy: invalid timestamp "czNjcjN0LXA0c3M="`
	secretDec := &Decision{
		PolicyName:   "p",
		ResourceKind: "Secret",
		Verdict:      VerdictTrace{Status: VerdictError},
	}
	list := NewDecisionList([]*Decision{secretDec}, &Summary{Error: 1}, errors.New(secretErr))
	raw, err := json.Marshal(list)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "czNjcjN0LXA0c3M=")
	var read DecisionList
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, redacted, read.Error)

	// A non-secret decision list keeps its error verbatim
	podDec := &Decision{
		PolicyName:   "p",
		ResourceKind: "Pod",
		Verdict:      VerdictTrace{Status: VerdictError},
	}
	nonSecretList := NewDecisionList([]*Decision{podDec}, &Summary{Error: 1}, errors.New("no such key: labels"))
	raw, err = json.Marshal(nonSecretList)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &read))
	assert.Equal(t, "no such key: labels", read.Error)
}

// TestLogEntry_FieldsAreADecisionsFields: a log entry is a decision with fields left out, so every
// field it writes is one a decision writes too, at the same place.
func TestLogEntry_FieldsAreADecisionsFields(t *testing.T) {
	d := fullDecision()
	entry, _ := logged(t, d)
	logged, err := json.Marshal(entry)
	require.NoError(t, err)
	written, err := json.Marshal(d)
	require.NoError(t, err)

	decisionPaths := jsonPaths(t, written)
	for path := range jsonPaths(t, logged) {
		if path == "apiVersion" || path == "kind" {
			continue
		}
		assert.Contains(t, decisionPaths, path, "a log entry field a decision does not have")
	}
}

// jsonPaths returns the path of every field in a JSON document, with list items under their list.
func jsonPaths(t *testing.T, raw []byte) map[string]struct{} {
	t.Helper()
	var doc any
	require.NoError(t, json.Unmarshal(raw, &doc))
	paths := map[string]struct{}{}
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for key, value := range v {
				path := key
				if prefix != "" {
					path = prefix + "." + key
				}
				paths[path] = struct{}{}
				walk(path, value)
			}
		case []any:
			for _, item := range v {
				walk(prefix+"[]", item)
			}
		}
	}
	walk("", doc)
	return paths
}
