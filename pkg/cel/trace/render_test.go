package trace

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRender_NilWritesNothing(t *testing.T) {
	var sb strings.Builder
	Render(&sb, nil)
	assert.Empty(t, sb.String())
}

func TestRender_FailingDecision(t *testing.T) {
	d := &Decision{
		PolicyName:        "require-labels",
		PolicyKind:        "ValidatingPolicy",
		ResourceKind:      "Pod",
		ResourceName:      "nginx",
		ResourceNamespace: "prod",
		Scope:             ScopeTrace{Applied: true, Reason: "matched kind Pod, namespace prod, operation CREATE"},
		Match: []NamedExpressionTrace{{
			Name:            "not-kube-system",
			ExpressionTrace: ExpressionTrace{Source: "object.metadata.namespace != 'kube-system'", Result: "true"},
		}},
		Variables: []NamedExpressionTrace{{
			Name:            "hasTeam",
			ExpressionTrace: ExpressionTrace{Source: "has(object.metadata.labels.team)", Result: "false"},
		}},
		Verdict: VerdictTrace{
			Status: VerdictFail,
			ExpressionTrace: ExpressionTrace{
				Source: "variables.hasTeam",
				Result: "false",
				Nodes:  []NodeTrace{{Expression: "variables.hasTeam", Value: "false"}},
			},
			Message: "every pod must carry a team label",
		},
	}

	var sb strings.Builder
	Render(&sb, d)
	out := sb.String()

	for _, want := range []string{
		"Policy:   require-labels (ValidatingPolicy)",
		"Resource: Pod/nginx (namespace: prod)",
		"SCOPE      applied  matched kind Pod, namespace prod, operation CREATE",
		"not-kube-system: object.metadata.namespace != 'kube-system'  ->  true",
		"hasTeam: has(object.metadata.labels.team)  ->  false",
		"VERDICT    FAIL     variables.hasTeam  ->  false",
		`message: "every pod must carry a team label"`,
		"evaluated:",
	} {
		assert.Contains(t, out, want)
	}
}

func TestRender_PassHidesNodeBreakdown(t *testing.T) {
	d := &Decision{
		Scope: ScopeTrace{Applied: true, Reason: "matched"},
		Verdict: VerdictTrace{
			Status:          VerdictPass,
			ExpressionTrace: ExpressionTrace{Source: "true", Result: "true", Nodes: []NodeTrace{{Expression: "true", Value: "true"}}},
		},
	}
	var sb strings.Builder
	Render(&sb, d)
	assert.NotContains(t, sb.String(), "evaluated:")
	assert.NotContains(t, sb.String(), "message:")
}

func TestRender_SkipAndErrorNodes(t *testing.T) {
	skip := &Decision{
		Scope:   ScopeTrace{Applied: false, Reason: "kind Pod is not covered by the policy's resourceRules"},
		Verdict: VerdictTrace{Status: VerdictSkip, Message: "the policy does not apply to this resource"},
	}
	var sb strings.Builder
	Render(&sb, skip)
	assert.Contains(t, sb.String(), "SCOPE      skipped")
	assert.Contains(t, sb.String(), "VERDICT    SKIP     the policy does not apply to this resource")

	errored := &Decision{
		Verdict: VerdictTrace{
			Status: VerdictError,
			ExpressionTrace: ExpressionTrace{
				Source: "object.a.b",
				Result: "no such key: a",
				Nodes:  []NodeTrace{{Expression: "object.a", Error: "no such key: a"}},
			},
			Message: "no such key: a",
		},
	}
	sb.Reset()
	Render(&sb, errored)
	assert.Contains(t, sb.String(), "object.a  ->  ERROR: no such key: a")
}

func TestRender_LongValuesAreClipped(t *testing.T) {
	long := strings.Repeat("x", 500)
	d := &Decision{Verdict: VerdictTrace{
		Status:          VerdictFail,
		ExpressionTrace: ExpressionTrace{Source: "expr", Result: long},
	}}
	var sb strings.Builder
	Render(&sb, d)
	assert.Less(t, len(sb.String()), 300)
	assert.Contains(t, sb.String(), "...")
}
