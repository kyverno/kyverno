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
	p, errs := NewCompiler(traced).Compile(policy, nil)
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
	result := compileAndEvaluate(t, true, policy, podObject("kube-system", map[string]any{"team": "a"}))

	// a skipped policy returns a nil result, so there is nothing to hang a trace on; this pins
	// the current behavior so a future change to it is deliberate
	assert.Nil(t, result)
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
			assert.Nil(t, on)
			continue
		}
		require.NotNil(t, on)
		assert.Equal(t, off.Result, on.Result)
		assert.Equal(t, off.Index, on.Index)
		assert.Equal(t, off.Message, on.Message)
		assert.Equal(t, off.Error != nil, on.Error != nil)
	}
}
