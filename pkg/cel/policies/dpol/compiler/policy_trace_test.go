package compiler

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// traceTestPolicy deletes a pod only when it carries an "expires" label and is tier=temp. The
// second condition reads labels.tier directly, so a pod with labels but no tier makes it error.
func traceTestPolicy() *policiesv1beta1.DeletingPolicy {
	return &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "delete-expired-pods"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule: "*/5 * * * *",
			Variables: []admissionregistrationv1.Variable{{
				Name:       "expiry",
				Expression: "object.metadata.?labels.?expires.orValue('')",
			}},
			Conditions: []admissionregistrationv1.MatchCondition{
				{Name: "has-expiry", Expression: "variables.expiry != ''"},
				{Name: "is-temporary", Expression: "object.metadata.labels.tier == 'temp'"},
			},
		},
	}
}

func tracePod(labels map[string]any) unstructured.Unstructured {
	metadata := map[string]any{"name": "pod", "namespace": "prod"}
	if labels != nil {
		metadata["labels"] = labels
	}
	return unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Pod", "metadata": metadata}}
}

func evaluateTraced(t *testing.T, traced bool, policy *policiesv1beta1.DeletingPolicy, exceptions []*policiesv1beta1.PolicyException, obj unstructured.Unstructured) (*EvaluationResult, error) {
	t.Helper()
	compiler := NewCompiler()
	if traced {
		compiler = NewCompilerWithTrace(true)
	}
	p, errs := compiler.Compile(policy, exceptions)
	require.Empty(t, errs)
	require.Equal(t, traced, p.Tracing())
	ns := unstructured.Unstructured{}
	return p.Evaluate(context.Background(), obj, &ns, &libs.FakeContextProvider{})
}

func TestEvaluate_Tracing(t *testing.T) {
	exemptProd := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "keep-prod", Namespace: "prod"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "in-prod", Expression: "object.metadata.namespace == 'prod'"}},
		},
	}
	tests := []struct {
		name        string
		labels      map[string]any
		exceptions  []*policiesv1beta1.PolicyException
		wantErr     bool
		wantStatus  string
		wantMessage string
		// wantConditions is name -> result for every condition the trace must record, in order
		wantConditions [][2]string
	}{{
		name:           "every condition holds",
		labels:         map[string]any{"expires": "2026-01-01", "tier": "temp"},
		wantStatus:     trace.VerdictPass,
		wantMessage:    "conditions held: the resource would be deleted",
		wantConditions: [][2]string{{"has-expiry", "true"}, {"is-temporary", "true"}},
	}, {
		// match stops at the first false condition, so the second one is never evaluated
		name:           "first condition false",
		labels:         map[string]any{"tier": "temp"},
		wantStatus:     trace.VerdictFail,
		wantMessage:    `condition "has-expiry" is false: the resource is kept`,
		wantConditions: [][2]string{{"has-expiry", "false"}},
	}, {
		name:           "condition errors",
		labels:         map[string]any{"expires": "2026-01-01"},
		wantErr:        true,
		wantStatus:     trace.VerdictError,
		wantMessage:    "no such key: tier",
		wantConditions: [][2]string{{"has-expiry", "true"}, {"is-temporary", "no such key: tier"}},
	}, {
		name:       "exempted by a policy exception",
		labels:     map[string]any{"expires": "2026-01-01", "tier": "temp"},
		exceptions: []*policiesv1beta1.PolicyException{exemptProd},
		// FAIL, not SKIP: the result is "not deleted", which the CLI reports as fail
		wantStatus:  trace.VerdictFail,
		wantMessage: "exempted by policy exception prod/keep-prod: the resource is kept",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			untraced, untracedErr := evaluateTraced(t, false, traceTestPolicy(), tt.exceptions, tracePod(tt.labels))
			traced, tracedErr := evaluateTraced(t, true, traceTestPolicy(), tt.exceptions, tracePod(tt.labels))

			// tracing must not change the outcome
			if tt.wantErr {
				require.Error(t, untracedErr)
				require.Error(t, tracedErr)
				assert.Equal(t, untracedErr.Error(), tracedErr.Error())
				assert.Nil(t, untraced, "untraced errors keep returning a nil result")
			} else {
				require.NoError(t, untracedErr)
				require.NoError(t, tracedErr)
				require.NotNil(t, untraced)
				assert.Equal(t, untraced.Result, traced.Result)
				assert.Equal(t, untraced.Exceptions, traced.Exceptions)
				assert.Nil(t, untraced.Trace, "no trace when compiled without tracing")
			}

			require.NotNil(t, traced)
			require.NotNil(t, traced.Trace)
			assert.Equal(t, tt.wantStatus, traced.Trace.Verdict.Status)
			assert.Equal(t, tt.wantMessage, traced.Trace.Verdict.Message)
			got := make([][2]string, 0, len(traced.Trace.Match))
			for _, c := range traced.Trace.Match {
				got = append(got, [2]string{c.Name, c.Result})
			}
			if len(tt.wantConditions) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.wantConditions, got)
			}
		})
	}
}

func TestEvaluate_TracingRecordsVariables(t *testing.T) {
	traced, err := evaluateTraced(t, true, traceTestPolicy(), nil, tracePod(map[string]any{"expires": "2026-01-01", "tier": "temp"}))
	require.NoError(t, err)
	require.Len(t, traced.Trace.Variables, 1)
	assert.Equal(t, "expiry", traced.Trace.Variables[0].Name)
	assert.Equal(t, "2026-01-01", traced.Trace.Variables[0].Result)
}

func TestEvaluate_TracingErrorNodeShowsFailingLookup(t *testing.T) {
	traced, err := evaluateTraced(t, true, traceTestPolicy(), nil, tracePod(map[string]any{"expires": "2026-01-01"}))
	require.Error(t, err)
	require.Len(t, traced.Trace.Match, 2)
	var sawError bool
	for _, n := range traced.Trace.Match[1].Nodes {
		if n.Error != "" {
			sawError = true
			assert.Empty(t, n.Value, "an errored node must not also carry a value")
		}
	}
	assert.True(t, sawError, "the failing condition should show which sub-expression errored")
}

// TestEvaluate_TracingMatchesUntracedOnAnExpensiveExpression: in cel-go v0.31 a program that
// tracks state does not enforce the per-call cost limit, which is why the decision always comes
// from the normal program and the tracking twin only explains. DeletingPolicy's environment sets
// no per-call cost limit today, so this condition holds either way; the test pins that tracing
// gives the same outcome, so if a limit is ever added, tracing cannot lift it.
func TestEvaluate_TracingMatchesUntracedOnAnExpensiveExpression(t *testing.T) {
	const items = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]"
	costly := items + ".all(a, " + items + ".all(b, " + items + ".all(c, " + items + ".all(d, " + items + ".all(e, a+b+c+d+e > 0)))))"
	policy := traceTestPolicy()
	policy.Spec.Conditions = []admissionregistrationv1.MatchCondition{{Name: "expensive", Expression: costly}}
	pod := tracePod(map[string]any{"tier": "temp"})

	untraced, untracedErr := evaluateTraced(t, false, policy, nil, pod)
	traced, tracedErr := evaluateTraced(t, true, policy, nil, pod)

	if untracedErr != nil {
		require.Error(t, tracedErr, "tracing must not lift a limit the untraced run hit")
		assert.Equal(t, untracedErr.Error(), tracedErr.Error())
		return
	}
	require.NoError(t, tracedErr)
	assert.Equal(t, untraced.Result, traced.Result)
}
