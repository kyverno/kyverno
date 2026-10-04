package trace

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestRender_MutatingPolicyPassMessage checks the source-less-verdict fallback message: a
// MutatingPolicy has no "validations" at all, so the vpol-oriented "no validations to evaluate"
// wording would be actively wrong here, not just imprecise. The distinguishing signal is
// Mutations being non-empty, not the policy kind string, so this holds for any future kind that
// also has no single deciding expression.
func TestRender_MutatingPolicyPassMessage(t *testing.T) {
	d := &Decision{
		PolicyKind: "MutatingPolicy",
		Mutations: []MutationTrace{
			{Name: "mutations[0] (applyConfiguration)", ExpressionTrace: ExpressionTrace{Source: "Object{}", Result: "map[]"}},
		},
		Verdict: VerdictTrace{Status: VerdictPass},
	}
	var sb strings.Builder
	Render(&sb, d)
	out := sb.String()
	assert.NotContains(t, out, "no validations to evaluate", "this wording is vpol-specific and wrong for a policy kind with no validations at all")
	assert.Contains(t, out, "MUTATIONS")
	assert.Contains(t, out, "VERDICT    PASS     completed; see MUTATIONS above for what ran",
		"the source-less mutating verdict must point at the MUTATIONS rows, not render empty")
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

// TestClip_DoesNotSplitAMultiByteRune builds a string where a multi-byte UTF-8 character (a
// 3-byte "€") straddles the maxValueLen byte offset, and checks clip backs up to a full
// character instead of cutting through the middle of one, which would produce invalid UTF-8.
func TestClip_DoesNotSplitAMultiByteRune(t *testing.T) {
	s := strings.Repeat("x", maxValueLen-1) + "€€€€€"
	require.Greater(t, len(s), maxValueLen, "the € characters must push the string past the clip point")

	got := clip(s)

	assert.True(t, utf8.ValidString(got), "clip produced invalid UTF-8: %q", got)
	assert.True(t, strings.HasSuffix(got, "..."), "clip should still append the ellipsis")
}

func TestRender_LoopValuesOmittedNote(t *testing.T) {
	const note = "values inside loops such as all() and exists() are not shown"
	loop := ExpressionTrace{
		Source:            "object.spec.containers.all(c, has(c.resources))",
		Result:            "false",
		Nodes:             []NodeTrace{{Expression: "object.spec.containers", Value: "[...]"}},
		LoopValuesOmitted: true,
	}
	var sb strings.Builder
	Render(&sb, &Decision{Verdict: VerdictTrace{Status: VerdictFail, ExpressionTrace: loop}})
	assert.Contains(t, sb.String(), note, "a failing loop must say why its body is not broken down")

	sb.Reset()
	Render(&sb, &Decision{Verdict: VerdictTrace{Status: VerdictPass, ExpressionTrace: loop}})
	assert.NotContains(t, sb.String(), note, "a pass shows no breakdown, so no note either")

	sb.Reset()
	loop.LoopValuesOmitted = false
	Render(&sb, &Decision{Verdict: VerdictTrace{Status: VerdictFail, ExpressionTrace: loop}})
	assert.NotContains(t, sb.String(), note)
}

func TestRender_ValidationsList(t *testing.T) {
	validation := func(index int, status, source, result string) ValidationTrace {
		return ValidationTrace{Index: index, Status: status, ExpressionTrace: ExpressionTrace{Source: source, Result: result}}
	}
	t.Run("a pass lists every validation and says all passed", func(t *testing.T) {
		var sb strings.Builder
		Render(&sb, &Decision{
			Validations: []ValidationTrace{validation(0, VerdictPass, "a", "true"), validation(1, VerdictPass, "b", "true")},
			Verdict:     VerdictTrace{Status: VerdictPass, ExpressionTrace: ExpressionTrace{Source: "b", Result: "true"}},
		})
		out := sb.String()
		assert.Contains(t, out, "VALIDATION PASS     [0] a  ->  true")
		assert.Contains(t, out, "VALIDATION PASS     [1] b  ->  true")
		assert.Contains(t, out, "VERDICT    PASS     all 2 validations passed")
		assert.NotContains(t, out, "VERDICT    PASS     b", "on a pass the last validation is not presented as the deciding one")
	})
	t.Run("a failure lists the rest as not run", func(t *testing.T) {
		var sb strings.Builder
		Render(&sb, &Decision{
			Validations: []ValidationTrace{
				validation(0, VerdictPass, "a", "true"),
				validation(1, VerdictFail, "b", "false"),
				validation(2, VerdictNotRun, "c", ""),
			},
			Verdict: VerdictTrace{Status: VerdictFail, ExpressionTrace: ExpressionTrace{Source: "b", Result: "false"}, Message: "needs b"},
		})
		out := sb.String()
		assert.Contains(t, out, "VALIDATION FAIL     [1] b  ->  false")
		assert.Contains(t, out, "VALIDATION NOT RUN  [2] c  (an earlier validation did not pass)")
		assert.Contains(t, out, "VERDICT    FAIL     b  ->  false")
		assert.Contains(t, out, `message: "needs b"`)
	})
	t.Run("a single validation is not listed", func(t *testing.T) {
		var sb strings.Builder
		Render(&sb, &Decision{
			Validations: []ValidationTrace{validation(0, VerdictPass, "a", "true")},
			Verdict:     VerdictTrace{Status: VerdictPass, ExpressionTrace: ExpressionTrace{Source: "a", Result: "true"}},
		})
		assert.NotContains(t, sb.String(), "VALIDATION")
		assert.Contains(t, sb.String(), "VERDICT    PASS     a  ->  true")
	})
}

func TestRender_Generations(t *testing.T) {
	d := &Decision{
		PolicyKind: "GeneratingPolicy",
		Generations: []GenerationTrace{
			{
				Name:            "generate[0] (expression)",
				ExpressionTrace: ExpressionTrace{Source: "generator.Apply(ns, [cm])", Result: "true"},
				Generated:       []string{"ConfigMap prod/zk-kafka-address"},
			},
			{Name: "generate[1] (template)"},
			{
				Name:            "generate[2] (expression)",
				ExpressionTrace: ExpressionTrace{Nodes: []NodeTrace{{Expression: "object.a", Error: "no such key: a"}}},
				Error:           "no such key: a",
			},
		},
		Verdict: VerdictTrace{Status: VerdictPass},
	}
	var sb strings.Builder
	Render(&sb, d)
	out := sb.String()
	for _, want := range []string{
		"GENERATE            generate[0] (expression): generator.Apply(ns, [cm])  ->  true",
		"generated ConfigMap prod/zk-kafka-address",
		"GENERATE            generate[1] (template)",
		"generated nothing",
		"GENERATE   ERROR    generate[2] (expression): no such key: a",
		"object.a  ->  ERROR: no such key: a",
		"VERDICT    PASS     completed; see GENERATE above for what ran",
	} {
		assert.Contains(t, out, want)
	}
	assert.NotContains(t, out, "no validations to evaluate")
}
