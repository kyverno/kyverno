package compiler

import (
	"context"
	"testing"

	v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	// generates ConfigMap <ns>/cm-from-expression into the trigger namespace
	traceGenerateExpression = `generator.Apply(variables.nsName, [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "cm-from-expression", "namespace": variables.nsName})}])`
	// fails at runtime on a trigger with no labels
	traceFailingExpression = `generator.Apply(variables.nsName, [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": string(object.metadata.labels.missing)})}])`
	traceGenerateTemplate  = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm-from-template
  namespace: (( variables.nsName ))
`
)

func traceTestPolicy(generations ...v1beta1.Generation) *v1beta1.GeneratingPolicy {
	return &v1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "generate-cms"},
		Spec: v1beta1.GeneratingPolicySpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "not-kube-system", Expression: "object.metadata.name != 'kube-system'"},
			},
			Variables: []admissionregistrationv1.Variable{
				{Name: "nsName", Expression: "object.metadata.name"},
			},
			Generation: generations,
		},
	}
}

// evaluateNamespace evaluates policy against a freshly built Namespace trigger, with its own
// context provider, so the traced and untraced runs share nothing.
func evaluateNamespace(t *testing.T, traced bool, policy *v1beta1.GeneratingPolicy, exceptions []*v1beta1.PolicyException, name string) (*EvaluationResult, error) {
	t.Helper()
	compiler := NewCompiler()
	if traced {
		compiler = NewCompilerWithTrace(true)
	}
	compiled, errs := compiler.Compile(policy, exceptions)
	require.Nil(t, errs)
	require.Equal(t, traced, compiled.Tracing())

	trigger := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": name}}}
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	ctxProvider := &libs.FakeContextProvider{}
	request := engine.Request(ctxProvider, gvk, gvr, "", name, "", admissionv1.Create, authenticationv1.UserInfo{}, trigger, nil, false, nil)
	attr := admission.NewAttributesRecord(trigger, nil, gvk, "", name, gvr, "", admission.Create, nil, false, &user.DefaultInfo{})
	return compiled.Evaluate(context.Background(), attr, &request.Request, &unstructured.Unstructured{}, ctxProvider)
}

func TestEvaluate_Tracing(t *testing.T) {
	exemptAll := &v1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "skip-generation", Namespace: "default"},
		Spec: v1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "always", Expression: "true"}},
		},
	}
	both := []v1beta1.Generation{
		{Expression: traceGenerateExpression},
		{Template: &v1beta1.GenerationTemplate{Interpolate: v1beta1.InterpolationModeCEL, Value: traceGenerateTemplate}},
	}
	tests := []struct {
		name        string
		generations []v1beta1.Generation
		exceptions  []*v1beta1.PolicyException
		trigger     string
		wantErr     bool
		wantSkipped bool
		wantStatus  string
		wantMessage string
		// wantGenerations is entry name -> what it generated, for every entry the trace records
		wantGenerations map[string][]string
	}{{
		name:        "expression and template both generate",
		generations: both,
		trigger:     "team-a",
		wantStatus:  trace.VerdictPass,
		wantMessage: "generated 2 resources",
		wantGenerations: map[string][]string{
			"generate[0] (expression)": {"ConfigMap team-a/cm-from-expression"},
			"generate[1] (template)":   {"ConfigMap team-a/cm-from-template"},
		},
	}, {
		name:        "match condition skips",
		generations: both,
		trigger:     "kube-system",
		wantSkipped: true,
		wantStatus:  trace.VerdictSkip,
		wantMessage: `match condition "not-kube-system" did not pass, so the policy was skipped`,
	}, {
		// the failing entry comes second, so the first one's output is still recorded
		name:        "a generate expression errors",
		generations: []v1beta1.Generation{{Expression: traceGenerateExpression}, {Expression: traceFailingExpression}},
		trigger:     "team-a",
		wantErr:     true,
		wantStatus:  trace.VerdictError,
		wantMessage: "no such key: labels",
		wantGenerations: map[string][]string{
			"generate[0] (expression)": {"ConfigMap team-a/cm-from-expression"},
			"generate[1] (expression)": nil,
		},
	}, {
		name:        "exempted by a policy exception",
		generations: both,
		exceptions:  []*v1beta1.PolicyException{exemptAll},
		trigger:     "team-a",
		wantStatus:  trace.VerdictSkip,
		wantMessage: "exempted by policy exception default/skip-generation",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			untraced, untracedErr := evaluateNamespace(t, false, traceTestPolicy(tt.generations...), tt.exceptions, tt.trigger)
			traced, tracedErr := evaluateNamespace(t, true, traceTestPolicy(tt.generations...), tt.exceptions, tt.trigger)

			// tracing must not change the outcome
			switch {
			case tt.wantErr:
				require.Error(t, untracedErr)
				require.Error(t, tracedErr)
				assert.Equal(t, untracedErr.Error(), tracedErr.Error())
				assert.Nil(t, untraced, "untraced errors keep returning a nil result")
			case tt.wantSkipped:
				require.NoError(t, untracedErr)
				require.NoError(t, tracedErr)
				assert.Nil(t, untraced, "an untraced skip keeps returning a nil result")
				require.NotNil(t, traced)
				assert.True(t, traced.Skipped)
				assert.Empty(t, traced.GeneratedResources)
			default:
				require.NoError(t, untracedErr)
				require.NoError(t, tracedErr)
				require.NotNil(t, untraced)
				assert.Equal(t, untraced.GeneratedResources, traced.GeneratedResources)
				assert.Equal(t, untraced.Exceptions, traced.Exceptions)
				assert.Nil(t, untraced.Trace, "no trace when compiled without tracing")
			}

			require.NotNil(t, traced)
			require.NotNil(t, traced.Trace)
			assert.Equal(t, tt.wantStatus, traced.Trace.Verdict.Status)
			assert.Contains(t, traced.Trace.Verdict.Message, tt.wantMessage)
			got := map[string][]string{}
			for _, g := range traced.Trace.Generations {
				got[g.Name] = g.Generated
			}
			if len(tt.wantGenerations) == 0 {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.wantGenerations, got)
			}
		})
	}
}

func TestEvaluate_TracingRecordsMatchVariablesAndErrorNode(t *testing.T) {
	traced, err := evaluateNamespace(t, true, traceTestPolicy(
		v1beta1.Generation{Expression: traceFailingExpression},
	), nil, "team-a")
	require.Error(t, err)

	require.Len(t, traced.Trace.Match, 1)
	assert.Equal(t, "not-kube-system", traced.Trace.Match[0].Name)
	assert.Equal(t, "true", traced.Trace.Match[0].Result)

	require.Len(t, traced.Trace.Variables, 1)
	assert.Equal(t, "nsName", traced.Trace.Variables[0].Name)
	assert.Equal(t, "team-a", traced.Trace.Variables[0].Result)

	require.Len(t, traced.Trace.Generations, 1)
	g := traced.Trace.Generations[0]
	assert.NotEmpty(t, g.Error)
	assert.Equal(t, traceFailingExpression, g.Source)
	var sawError bool
	for _, n := range g.Nodes {
		if n.Error != "" {
			sawError = true
			assert.Empty(t, n.Value, "an errored node must not also carry a value")
		}
	}
	assert.True(t, sawError, "the failing generate expression should show which sub-expression errored")
}

// TestEvaluate_TracingErrorPaths covers the error exits of Evaluate that the outcome table above
// does not reach. Each must return exactly what it returned before tracing existed (a nil result
// and the error) when tracing is off, and the same error plus the partial trace when it is on.
func TestEvaluate_TracingErrorPaths(t *testing.T) {
	failingTemplate := v1beta1.Generation{Template: &v1beta1.GenerationTemplate{
		Interpolate: v1beta1.InterpolationModeCEL,
		Value: `
apiVersion: v1
kind: ConfigMap
metadata:
  name: (( string(object.metadata.labels.missing) ))
  namespace: (( variables.nsName ))
`,
	}}
	tests := []struct {
		name       string
		policy     func() *v1beta1.GeneratingPolicy
		exceptions []*v1beta1.PolicyException
		check      func(t *testing.T, d *trace.Decision)
	}{{
		name: "a template fails to render",
		policy: func() *v1beta1.GeneratingPolicy {
			return traceTestPolicy(v1beta1.Generation{Expression: traceGenerateExpression}, failingTemplate)
		},
		check: func(t *testing.T, d *trace.Decision) {
			require.Len(t, d.Generations, 2)
			assert.Equal(t, []string{"ConfigMap team-a/cm-from-expression"}, d.Generations[0].Generated, "the entry before the failure keeps what it generated")
			assert.Equal(t, "generate[1] (template)", d.Generations[1].Name)
			assert.NotEmpty(t, d.Generations[1].Error)
			assert.Empty(t, d.Generations[1].Generated)
		},
	}, {
		name: "an exception's match condition errors",
		policy: func() *v1beta1.GeneratingPolicy {
			return traceTestPolicy(v1beta1.Generation{Expression: traceGenerateExpression})
		},
		exceptions: []*v1beta1.PolicyException{{
			ObjectMeta: metav1.ObjectMeta{Name: "broken", Namespace: "default"},
			Spec: v1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "broken", Expression: "object.metadata.labels.missing == 'x'"}},
			},
		}},
		check: func(t *testing.T, d *trace.Decision) {
			// exceptions are checked first, so nothing else has run yet
			assert.Empty(t, d.Match)
			assert.Empty(t, d.Generations)
		},
	}, {
		name: "an audit annotation errors after generation",
		policy: func() *v1beta1.GeneratingPolicy {
			p := traceTestPolicy(v1beta1.Generation{Expression: traceGenerateExpression})
			p.Spec.AuditAnnotations = []admissionregistrationv1.AuditAnnotation{{Key: "owner", ValueExpression: "string(object.metadata.labels.missing)"}}
			return p
		},
		check: func(t *testing.T, d *trace.Decision) {
			require.Len(t, d.Generations, 1)
			assert.Equal(t, []string{"ConfigMap team-a/cm-from-expression"}, d.Generations[0].Generated)
			assert.Empty(t, d.Generations[0].Error, "the generate entry itself succeeded")
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			untraced, untracedErr := evaluateNamespace(t, false, tt.policy(), tt.exceptions, "team-a")
			traced, tracedErr := evaluateNamespace(t, true, tt.policy(), tt.exceptions, "team-a")

			require.Error(t, untracedErr)
			require.Error(t, tracedErr)
			assert.Equal(t, untracedErr.Error(), tracedErr.Error(), "tracing must not change the error")
			assert.Nil(t, untraced, "untraced errors keep returning a nil result")

			require.NotNil(t, traced)
			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictError, traced.Trace.Verdict.Status)
			assert.Equal(t, tracedErr.Error(), traced.Trace.Verdict.Message)
			tt.check(t, traced.Trace)
		})
	}
}

// TestEvaluate_TracingDoesNotGenerateTwice: to explain a decision each expression's tracking twin
// is re-run, and for a GeneratingPolicy that re-run would call generator.Apply again. It runs
// against a generator that generates nothing, so a traced run generates exactly what an untraced
// one does, while the generate expression still gets its node breakdown. Covers generator.Apply
// called from a generate entry and from a variable the entry reads.
func TestEvaluate_TracingDoesNotGenerateTwice(t *testing.T) {
	const want = "ConfigMap team-a/cm-from-expression"
	tests := map[string]*v1beta1.GeneratingPolicy{
		"from a generate expression": traceTestPolicy(v1beta1.Generation{Expression: traceGenerateExpression}),
		"from a variable": func() *v1beta1.GeneratingPolicy {
			p := traceTestPolicy(v1beta1.Generation{Expression: "variables.generated"})
			p.Spec.Variables = append(p.Spec.Variables, admissionregistrationv1.Variable{Name: "generated", Expression: traceGenerateExpression})
			return p
		}(),
	}
	for name, policy := range tests {
		t.Run(name, func(t *testing.T) {
			untraced, err := evaluateNamespace(t, false, policy, nil, "team-a")
			require.NoError(t, err)
			traced, err := evaluateNamespace(t, true, policy, nil, "team-a")
			require.NoError(t, err)

			require.Len(t, untraced.GeneratedResources, 1)
			require.Len(t, traced.GeneratedResources, 1, "explaining must not generate the resource a second time")
			assert.Equal(t, untraced.GeneratedResources[0].Object, traced.GeneratedResources[0].Object)

			require.NotNil(t, traced.Trace)
			assert.Equal(t, trace.VerdictPass, traced.Trace.Verdict.Status)
			assert.Equal(t, "generated 1 resource", traced.Trace.Verdict.Message)
			require.Len(t, traced.Trace.Generations, 1)
			assert.Equal(t, []string{want}, traced.Trace.Generations[0].Generated)
			assert.Equal(t, "true", traced.Trace.Generations[0].Result)
			assert.NotEmpty(t, traced.Trace.Generations[0].Nodes, "the re-run still explains the generate expression")
		})
	}
}

// TestEvaluate_TracingMatchesUntracedOnAnExpensiveExpression: in cel-go v0.31 a program that
// tracks state does not enforce the per-call cost limit, which is why the decision always comes
// from the normal program and the tracking twin only explains. GeneratingPolicy's environment
// sets no per-call cost limit today, so this expression succeeds either way; the test pins that
// tracing gives the same outcome, so if a limit is ever added, tracing cannot lift it.
func TestEvaluate_TracingMatchesUntracedOnAnExpensiveExpression(t *testing.T) {
	const items = "[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]"
	costly := items + ".all(a, " + items + ".all(b, " + items + ".all(c, " + items + ".all(d, " + items + ".all(e, a+b+c+d+e > 0)))))"
	policy := traceTestPolicy(v1beta1.Generation{Expression: costly + " && " + traceGenerateExpression})

	untraced, untracedErr := evaluateNamespace(t, false, policy, nil, "team-a")
	traced, tracedErr := evaluateNamespace(t, true, policy, nil, "team-a")

	if untracedErr != nil {
		require.Error(t, tracedErr, "tracing must not lift a limit the untraced run hit")
		assert.Equal(t, untracedErr.Error(), tracedErr.Error())
		return
	}
	require.NoError(t, tracedErr)
	assert.Len(t, traced.GeneratedResources, len(untraced.GeneratedResources))
}
