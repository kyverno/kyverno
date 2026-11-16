package compiler

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func buildTracePolicy(validations ...admissionregistrationv1.Validation) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "trace-test"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "not-kube-system",
				Expression: "object.metadata.namespace != 'kube-system'",
			}},
			Variables: []admissionregistrationv1.Variable{{
				Name:       "hasTeam",
				Expression: "has(object.metadata.labels) && 'team' in object.metadata.labels",
			}},
			Validations: validations,
		},
	}
}

func threeValidations() []admissionregistrationv1.Validation {
	return []admissionregistrationv1.Validation{
		{Expression: "variables.hasTeam", Message: "needs team"},
		{Expression: "has(object.metadata.labels) && 'app' in object.metadata.labels", Message: "needs app"},
		{Expression: "object.metadata.namespace == 'prod'", Message: "must be prod"},
	}
}

func podObject(namespace string, labels map[string]any) map[string]any {
	metadata := map[string]any{"namespace": namespace}
	if labels != nil {
		metadata["labels"] = labels
	}
	return map[string]any{"metadata": metadata}
}

func compileAndEvaluate(t *testing.T, traced bool, policy *policiesv1beta1.ValidatingPolicy, object map[string]any) *EvaluationResult {
	t.Helper()
	p, errs := NewCompilerWithTrace(traced).Compile(policy, nil)
	require.Empty(t, errs)
	result, err := p.Evaluate(context.Background(), object, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	return result
}

func TestEvaluate_TracingOff_NoTrace(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)

	pass := compileAndEvaluate(t, false, policy, podObject("prod", map[string]any{"team": "a", "app": "b"}))
	require.NotNil(t, pass)
	assert.True(t, pass.Result)
	assert.Nil(t, pass.Trace, "no trace when the policy was compiled without tracing")

	fail := compileAndEvaluate(t, false, policy, podObject("prod", map[string]any{"team": "a"}))
	require.NotNil(t, fail)
	assert.False(t, fail.Result)
	assert.Equal(t, 1, fail.Index)
	assert.Equal(t, "needs app", fail.Message)
	assert.Nil(t, fail.Trace)
}

func TestEvaluate_TracingOn_Pass(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)
	result := compileAndEvaluate(t, true, policy, podObject("prod", map[string]any{"team": "a", "app": "b"}))

	require.NotNil(t, result)
	assert.True(t, result.Result)
	require.NotNil(t, result.Trace)

	// with everything passing, the verdict is the last validation evaluated
	assert.Equal(t, trace.VerdictPass, result.Trace.Verdict.Status)
	assert.Equal(t, "object.metadata.namespace == 'prod'", result.Trace.Verdict.Source)
	assert.Equal(t, "true", result.Trace.Verdict.Result)

	require.Len(t, result.Trace.Match, 1)
	assert.Equal(t, "not-kube-system", result.Trace.Match[0].Name)
	assert.Equal(t, "object.metadata.namespace != 'kube-system'", result.Trace.Match[0].Source)
	assert.Equal(t, "true", result.Trace.Match[0].Result)

	require.Len(t, result.Trace.Variables, 1)
	assert.Equal(t, "hasTeam", result.Trace.Variables[0].Name)
	assert.Equal(t, "true", result.Trace.Variables[0].Result)
}

func TestEvaluate_TracingOn_FailStopsAtFailingValidation(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)
	result := compileAndEvaluate(t, true, policy, podObject("prod", map[string]any{"team": "a"}))

	require.NotNil(t, result)
	assert.False(t, result.Result)
	assert.Equal(t, 1, result.Index)
	require.NotNil(t, result.Trace)

	verdict := result.Trace.Verdict
	assert.Equal(t, trace.VerdictFail, verdict.Status)
	assert.Equal(t, "needs app", verdict.Message)
	assert.Equal(t, "has(object.metadata.labels) && 'app' in object.metadata.labels", verdict.Source)
	assert.Equal(t, "false", verdict.Result)
	assert.NotEmpty(t, verdict.Nodes, "a failing verdict should carry its per-node breakdown")

	// the failing expression's sub-nodes show what the labels actually held
	var sawLabels bool
	for _, n := range verdict.Nodes {
		if n.Expression == "object.metadata.labels" {
			sawLabels = true
			assert.Contains(t, n.Value, "team")
		}
	}
	assert.True(t, sawLabels, "expected object.metadata.labels among the traced nodes")

	// validation 0 read the variable, so it was evaluated and traced
	require.Len(t, result.Trace.Variables, 1)
	assert.Equal(t, "true", result.Trace.Variables[0].Result)
}

func TestEvaluate_TracingOn_ErrorIsCapturedOnNode(t *testing.T) {
	policy := buildTracePolicy(admissionregistrationv1.Validation{
		Expression: "object.metadata.labels.team == 'a'",
		Message:    "needs team",
	})
	// no labels at all, so the field access errors instead of returning false
	result := compileAndEvaluate(t, true, policy, podObject("prod", nil))

	require.NotNil(t, result)
	require.Error(t, result.Error)
	require.NotNil(t, result.Trace)
	assert.Equal(t, trace.VerdictError, result.Trace.Verdict.Status)

	var sawError bool
	for _, n := range result.Trace.Verdict.Nodes {
		if n.Error != "" {
			sawError = true
			assert.Empty(t, n.Value, "an errored node must not also carry a value")
		}
	}
	assert.True(t, sawError, "expected at least one node with Error populated")
}

func TestEvaluate_TracingOn_MatchConditionFalseSkipsPolicy(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)
	object := podObject("kube-system", map[string]any{"team": "a"})

	// with tracing on, a skip is a non-nil result so the match trace is not lost; it is flagged
	// Skipped and is not a failure
	traced := compileAndEvaluate(t, true, policy, object)
	require.NotNil(t, traced)
	assert.True(t, traced.Skipped)
	assert.False(t, traced.Result)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictSkip, traced.Trace.Verdict.Status)
	assert.Contains(t, traced.Trace.Verdict.Message, "not-kube-system")
	require.Len(t, traced.Trace.Match, 1)
	assert.Equal(t, "false", traced.Trace.Match[0].Result)
	assert.Empty(t, traced.Trace.Variables, "no variable is read once a match condition excludes the resource")

	// with tracing off the behavior is unchanged: a skip is a nil result
	assert.Nil(t, compileAndEvaluate(t, false, policy, object))
}

func TestEvaluate_TracingOn_MatchConditionErrorKeepsMatchTraces(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)
	// the second condition reads a label that is absent, a runtime error; with the default
	// failurePolicy (Fail) that error is returned rather than treated as a non-match
	policy.Spec.MatchConditions = append(policy.Spec.MatchConditions, admissionregistrationv1.MatchCondition{
		Name:       "owner-is-platform",
		Expression: "object.metadata.labels.owner == 'platform'",
	})
	object := podObject("prod", nil)

	evaluate := func(traced bool) (*EvaluationResult, error) {
		p, errs := NewCompilerWithTrace(traced).Compile(policy, nil)
		require.Empty(t, errs)
		return p.Evaluate(context.Background(), object, nil, nil, nil, nil, nil)
	}

	// tracing off: unchanged, a nil result and the error
	untraced, untracedErr := evaluate(false)
	require.Error(t, untracedErr)
	assert.Nil(t, untraced)

	// tracing on: the same error, plus the match traces recorded up to the failure
	traced, tracedErr := evaluate(true)
	require.Error(t, tracedErr)
	assert.Equal(t, untracedErr.Error(), tracedErr.Error())
	require.NotNil(t, traced)
	require.NotNil(t, traced.Trace)
	assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
	assert.Equal(t, tracedErr.Error(), traced.Trace.Verdict.Message)

	require.Len(t, traced.Trace.Match, 2)
	assert.Equal(t, "not-kube-system", traced.Trace.Match[0].Name)
	assert.Equal(t, "true", traced.Trace.Match[0].Result)
	failing := traced.Trace.Match[1]
	assert.Equal(t, "owner-is-platform", failing.Name)
	var sawError bool
	for _, n := range failing.Nodes {
		if n.Error != "" {
			sawError = true
		}
	}
	assert.True(t, sawError, "the failing condition should show which sub-expression errored")
	assert.Empty(t, traced.Trace.Variables, "nothing past the match conditions runs")
}

// TestEvaluate_TracingOn_EveryValidationIsListed: each validation that ran is listed with its own
// status, and the ones after a failure or error are listed as not run, without being evaluated.
func TestEvaluate_TracingOn_EveryValidationIsListed(t *testing.T) {
	type entry struct{ status, result string }
	tests := []struct {
		name        string
		validations []admissionregistrationv1.Validation
		object      map[string]any
		verdict     string
		want        []entry
	}{{
		name:        "all pass",
		validations: threeValidations(),
		object:      podObject("prod", map[string]any{"team": "a", "app": "b"}),
		verdict:     trace.VerdictPass,
		want:        []entry{{trace.VerdictPass, "true"}, {trace.VerdictPass, "true"}, {trace.VerdictPass, "true"}},
	}, {
		name:        "the second fails, the third is not run",
		validations: threeValidations(),
		object:      podObject("prod", map[string]any{"team": "a"}),
		verdict:     trace.VerdictFail,
		want:        []entry{{trace.VerdictPass, "true"}, {trace.VerdictFail, "false"}, {trace.VerdictNotRun, ""}},
	}, {
		name: "the first errors, the rest are not run",
		validations: append([]admissionregistrationv1.Validation{
			{Expression: "object.metadata.labels.owner == 'platform'", Message: "needs owner"},
		}, threeValidations()[1:]...),
		object:  podObject("prod", nil),
		verdict: trace.VerdictError,
		want:    []entry{{trace.VerdictError, ""}, {trace.VerdictNotRun, ""}, {trace.VerdictNotRun, ""}},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := buildTracePolicy(tt.validations...)
			result := compileAndEvaluate(t, true, policy, tt.object)
			require.NotNil(t, result)
			require.NotNil(t, result.Trace)
			assert.Equal(t, tt.verdict, result.Trace.Verdict.Status)

			require.Len(t, result.Trace.Validations, len(tt.validations), "every validation is listed")
			for i, got := range result.Trace.Validations {
				assert.Equal(t, i, got.Index)
				assert.Equal(t, tt.want[i].status, got.Status, "validation %d", i)
				assert.Equal(t, tt.validations[i].Expression, got.Source, "validation %d", i)
				switch got.Status {
				case trace.VerdictNotRun:
					assert.Empty(t, got.Result, "a validation that never ran has no result")
					assert.Empty(t, got.Nodes)
				case trace.VerdictError:
					assert.NotEmpty(t, got.Result, "the error is the result")
				default:
					assert.Equal(t, tt.want[i].result, got.Result, "validation %d", i)
				}
			}
		})
	}
}

// TestEvaluate_TracingOn_IgnoredMatchErrorNamesTheRightCondition: with failurePolicy Ignore a
// match condition that errors does not stop the loop, so every condition runs and the policy is
// skipped afterwards. The skip message must name the condition that errored (or the one that came
// out false), not whichever condition happened to be evaluated last.
func TestEvaluate_TracingOn_IgnoredMatchErrorNamesTheRightCondition(t *testing.T) {
	ignore := admissionregistrationv1.Ignore
	errored := admissionregistrationv1.MatchCondition{Name: "owner-is-platform", Expression: "object.metadata.labels.owner == 'platform'"}
	alsoErrors := admissionregistrationv1.MatchCondition{Name: "tier-is-web", Expression: "object.metadata.labels.tier == 'web'"}
	passes := admissionregistrationv1.MatchCondition{Name: "passes", Expression: "true"}
	excludes := admissionregistrationv1.MatchCondition{Name: "excludes", Expression: "false"}
	tests := []struct {
		name       string
		conditions []admissionregistrationv1.MatchCondition
		want       string
		notWant    string
	}{{
		name:       "an errored condition followed by a passing one",
		conditions: []admissionregistrationv1.MatchCondition{errored, passes},
		want:       `match condition "owner-is-platform" failed to evaluate and failurePolicy is Ignore`,
		notWant:    `"passes"`,
	}, {
		name:       "several errored conditions are all named",
		conditions: []admissionregistrationv1.MatchCondition{errored, passes, alsoErrors},
		want:       `match conditions "owner-is-platform", "tier-is-web" failed to evaluate and failurePolicy is Ignore`,
		notWant:    `"passes"`,
	}, {
		name:       "a false condition decides even after an ignored error",
		conditions: []admissionregistrationv1.MatchCondition{errored, excludes},
		want:       `match condition "excludes" did not pass`,
		notWant:    "failurePolicy",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := buildTracePolicy(threeValidations()...)
			policy.Spec.FailurePolicy = &ignore
			policy.Spec.MatchConditions = append(policy.Spec.MatchConditions, tt.conditions...)
			object := podObject("prod", nil)

			assert.Nil(t, compileAndEvaluate(t, false, policy, object), "tracing off: an ignored error is still a plain skip")

			traced := compileAndEvaluate(t, true, policy, object)
			require.NotNil(t, traced)
			assert.True(t, traced.Skipped)
			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictSkip, traced.Trace.Verdict.Status)
			assert.Contains(t, traced.Trace.Verdict.Message, tt.want)
			assert.NotContains(t, traced.Trace.Verdict.Message, tt.notWant)
			assert.Len(t, traced.Trace.Match, 1+len(tt.conditions), "every condition runs under Ignore and is traced")
		})
	}
}

func TestEvaluate_TracingOn_MatchesTracingOffOutcome(t *testing.T) {
	policy := buildTracePolicy(threeValidations()...)
	objects := []map[string]any{
		podObject("prod", map[string]any{"team": "a", "app": "b"}),
		podObject("prod", map[string]any{"team": "a"}),
		podObject("prod", nil),
		podObject("dev", map[string]any{"team": "a", "app": "b"}),
		podObject("kube-system", map[string]any{"team": "a"}),
	}
	for _, object := range objects {
		off := compileAndEvaluate(t, false, policy, object)
		on := compileAndEvaluate(t, true, policy, object)
		if off == nil {
			require.NotNil(t, on)
			assert.True(t, on.Skipped, "a policy skipped without tracing must be reported as skipped with it")
			continue
		}
		require.NotNil(t, on)
		assert.Equal(t, off.Result, on.Result)
		assert.Equal(t, off.Index, on.Index)
		assert.Equal(t, off.Message, on.Message)
		assert.Equal(t, off.Error != nil, on.Error != nil)
	}
}

// TestEvaluate_TracingKeepsTheCostLimit is the review repro for --explain turning a cost-limit
// error into a decision: state tracking disables cost-limit enforcement in cel-go, so the
// expression must be decided by the untracked program in every position it can appear, and the
// trace must report the same error rather than an outcome the untraced run never reaches.
func TestEvaluate_TracingKeepsTheCostLimit(t *testing.T) {
	const items = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]"
	costly := items + ".all(a, " + items + ".all(b, " + items + ".all(c, " + items + ".all(d, " + items + ".all(e, a+b+c+d+e > 0)))))"
	tests := map[string]func(p *policiesv1beta1.ValidatingPolicy){
		"in a validation": func(p *policiesv1beta1.ValidatingPolicy) {
			p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: costly}}
		},
		"in a variable": func(p *policiesv1beta1.ValidatingPolicy) {
			p.Spec.Variables = []admissionregistrationv1.Variable{{Name: "everything", Expression: costly}}
			p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "variables.everything"}}
		},
		"in a match condition": func(p *policiesv1beta1.ValidatingPolicy) {
			p.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{Name: "expensive", Expression: costly}}
			p.Spec.Validations = []admissionregistrationv1.Validation{{Expression: "true"}}
		},
	}
	for name, configure := range tests {
		t.Run(name, func(t *testing.T) {
			policy := buildTracePolicy()
			configure(policy)
			evaluate := func(traced bool) (*EvaluationResult, error) {
				compiler := NewCompiler()
				if traced {
					compiler = NewCompilerWithTrace(true)
				}
				p, errs := compiler.Compile(policy, nil)
				require.Empty(t, errs)
				return p.Evaluate(context.Background(), podObject("prod", map[string]any{"team": "a"}), nil, nil, nil, nil, nil)
			}
			// the error surfaces as Evaluate's error (match condition) or as result.Error (the rest)
			outcome := func(result *EvaluationResult, err error) error {
				if err != nil {
					return err
				}
				require.NotNil(t, result)
				return result.Error
			}

			untraced, untracedErr := evaluate(false)
			traced, tracedErr := evaluate(true)
			untracedOutcome := outcome(untraced, untracedErr)
			tracedOutcome := outcome(traced, tracedErr)

			require.Error(t, untracedOutcome, "the cost limit must stop the expression")
			require.Error(t, tracedOutcome, "tracing must not lift the cost limit")
			assert.Equal(t, untracedOutcome.Error(), tracedOutcome.Error())
			assert.Contains(t, tracedOutcome.Error(), "cost limit exceeded")

			require.NotNil(t, traced)
			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
			assert.Contains(t, traced.Trace.Verdict.Message, "cost limit exceeded")
		})
	}
}
