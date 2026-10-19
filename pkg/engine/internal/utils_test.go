package internal

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildStatementMapRejectsMalformedType is the harden-type-handling
// regression: remote/attacker-controlled statement JSON with a non-string or
// missing "type" field must produce an error, never panic.
func TestBuildStatementMapRejectsMalformedType(t *testing.T) {
	tests := []struct {
		name       string
		statements []map[string]any
	}{
		{
			name: "type is a number",
			statements: []map[string]any{
				{"type": 1337},
			},
		},
		{
			name: "type is missing",
			statements: []map[string]any{
				{"predicate": map[string]any{}},
			},
		},
		{
			name: "type is a nested object",
			statements: []map[string]any{
				{"type": map[string]any{"nested": true}},
			},
		},
		{
			name: "type is nil",
			statements: []map[string]any{
				{"type": nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotPanics(t, func() {
				_, _, err := buildStatementMap(tt.statements)
				assert.Error(t, err)
			})
		})
	}
}

// TestBuildStatementMapGroupsByType confirms the well-formed path still
// groups statements by predicate type correctly, so the hardening above
// doesn't change behavior for valid input.
func TestBuildStatementMapGroupsByType(t *testing.T) {
	statements := []map[string]any{
		{"type": "type-a", "value": 1},
		{"type": "type-b", "value": 2},
		{"type": "type-a", "value": 3},
	}

	results, types, err := buildStatementMap(statements)
	require.NoError(t, err)
	assert.Equal(t, []string{"type-a", "type-b", "type-a"}, types)
	assert.Len(t, results["type-a"], 2)
	assert.Len(t, results["type-b"], 1)
}

func Test_EvaluateConditions_PredicateIsolation(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := enginecontext.NewContext(jp)
	log := logr.Discard()

	stmtA := map[string]any{
		"predicate": map[string]any{
			"onlyInA": "x",
		},
	}
	condA := []kyvernov1.AnyAllConditions{
		{
			AllConditions: []kyvernov1.Condition{
				{
					RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ onlyInA }}"`)},
					Operator: kyvernov1.ConditionOperators["Equals"],
					RawValue: &apiextv1.JSON{Raw: []byte(`"x"`)},
				},
			},
		},
	}
	passA, _, err := EvaluateConditions(condA, ctx, stmtA, log)
	assert.NoError(t, err)
	assert.True(t, passA)

	stmtB := map[string]any{
		"predicate": map[string]any{
			"other": "y",
		},
	}
	condB := []kyvernov1.AnyAllConditions{
		{
			AllConditions: []kyvernov1.Condition{
				{
					RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ onlyInA || '' }}"`)},
					Operator: kyvernov1.ConditionOperators["Equals"],
					RawValue: &apiextv1.JSON{Raw: []byte(`"x"`)},
				},
			},
		},
	}
	passB, _, err := EvaluateConditions(condB, ctx, stmtB, log)
	assert.NoError(t, err)
	assert.False(t, passB, "statement B condition must not see onlyInA from statement A")

	res, err := ctx.Query("onlyInA")
	assert.True(t, err != nil || res == nil, "onlyInA must not remain in parent context")
}

func Test_EvaluateConditions_ErrorPathRestoresContext(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := enginecontext.NewContext(jp)
	log := logr.Discard()

	stmtErr := map[string]any{
		"predicate": map[string]any{
			"onlyInErr": "val",
		},
	}
	condErr := []kyvernov1.AnyAllConditions{
		{
			AllConditions: []kyvernov1.Condition{
				{
					RawKey:   &apiextv1.JSON{Raw: []byte(`"{{ 1 + }}"`)},
					Operator: kyvernov1.ConditionOperators["Equals"],
					RawValue: &apiextv1.JSON{Raw: []byte(`"x"`)},
				},
			},
		},
	}
	_, _, err := EvaluateConditions(condErr, ctx, stmtErr, log)
	assert.Error(t, err)

	res, err := ctx.Query("onlyInErr")
	assert.True(t, err != nil || res == nil, "deferred Restore must run and clear context on error path")
}
