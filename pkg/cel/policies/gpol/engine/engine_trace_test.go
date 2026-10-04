package engine

import (
	"testing"

	v1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestHandle_Tracing drives Handle with a real matcher for each outcome and checks the trace the
// engine attaches: the header, SCOPE, the verdict (which must follow the rule result, including an
// exception whose reportResult is pass), and that tracing off leaves the response unchanged.
func TestHandle_Tracing(t *testing.T) {
	const generate = `generator.Apply(object.metadata.name, [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "cm", "namespace": object.metadata.name})}])`
	newPolicy := func(resource string, conditions ...admissionregistrationv1.MatchCondition) *v1beta1.GeneratingPolicy {
		return &v1beta1.GeneratingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "generate-cm"},
			Spec: v1beta1.GeneratingPolicySpec{
				MatchConstraints: &admissionregistrationv1.MatchResources{
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{resource},
							},
						},
					}},
				},
				MatchConditions: conditions,
				Generation:      []v1beta1.Generation{{Expression: generate}},
			},
		}
	}
	exception := func(reportResult string) *v1beta1.PolicyException {
		return &v1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "polex", Namespace: "default"},
			Spec: v1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "always", Expression: "true"}},
				ReportResult:    reportResult,
			},
		}
	}
	tests := []struct {
		name        string
		policy      *v1beta1.GeneratingPolicy
		exceptions  []*v1beta1.PolicyException
		wantRule    engineapi.RuleStatus // empty means no rule result at all
		wantApplied bool
		wantStatus  string
		wantMessage string
	}{{
		name:        "generates",
		policy:      newPolicy("namespaces"),
		wantRule:    engineapi.RuleStatusPass,
		wantApplied: true,
		wantStatus:  trace.VerdictPass,
		wantMessage: "generated 1 resource",
	}, {
		name:        "matchConstraints do not cover the trigger",
		policy:      newPolicy("configmaps"),
		wantStatus:  trace.VerdictSkip,
		wantMessage: "the policy does not apply to this resource",
	}, {
		name:        "match condition skips",
		policy:      newPolicy("namespaces", admissionregistrationv1.MatchCondition{Name: "never", Expression: "false"}),
		wantApplied: true,
		wantStatus:  trace.VerdictSkip,
		wantMessage: `match condition "never" did not pass`,
	}, {
		name:        "exception reported as skip",
		policy:      newPolicy("namespaces"),
		exceptions:  []*v1beta1.PolicyException{exception("")},
		wantRule:    engineapi.RuleStatusSkip,
		wantApplied: true,
		wantStatus:  trace.VerdictSkip,
		wantMessage: "rule is skipped due to policy exception: default/polex",
	}, {
		name:        "exception reported as pass",
		policy:      newPolicy("namespaces"),
		exceptions:  []*v1beta1.PolicyException{exception(string(engineapi.RuleStatusPass))},
		wantRule:    engineapi.RuleStatusPass,
		wantApplied: true,
		wantStatus:  trace.VerdictPass,
		wantMessage: "rule is passed due to policy exception: default/polex",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := func(traced bool) GeneratingPolicyResponse {
				compiled, errs := compiler.NewCompilerWithTrace(traced).Compile(tt.policy, tt.exceptions)
				require.Nil(t, errs)
				trigger := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "team-a"},
				}}
				req := engine.Request(
					libs.NewFakeContextProvider(),
					schema.GroupVersionKind{Version: "v1", Kind: "Namespace"},
					schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
					"", "team-a", "", admissionv1.Create, authenticationv1.UserInfo{}, trigger, nil, false, nil,
				)
				eng := NewEngine(func(string) *corev1.Namespace { return nil }, matching.NewMatcher())
				resp, err := eng.Handle(req, Policy{Policy: tt.policy, Exceptions: tt.exceptions, CompiledPolicy: compiled}, false)
				require.NoError(t, err)
				require.Len(t, resp.Policies, 1)
				return resp.Policies[0]
			}

			untraced := handle(false)
			traced := handle(true)

			// tracing must not change the rule result
			if tt.wantRule == "" {
				assert.Nil(t, untraced.Result)
				assert.Nil(t, traced.Result)
			} else {
				require.NotNil(t, untraced.Result)
				require.NotNil(t, traced.Result)
				assert.Equal(t, tt.wantRule, untraced.Result.Status())
				assert.Equal(t, untraced.Result.Status(), traced.Result.Status())
				assert.Equal(t, untraced.Result.Message(), traced.Result.Message())
				assert.Equal(t, len(untraced.Result.GeneratedResources()), len(traced.Result.GeneratedResources()))
			}
			assert.Nil(t, untraced.Trace, "no trace when compiled without tracing")

			got := traced.Trace
			require.NotNil(t, got)
			assert.Equal(t, "generate-cm", got.PolicyName)
			assert.Equal(t, "GeneratingPolicy", got.PolicyKind)
			assert.Equal(t, "Namespace", got.ResourceKind)
			assert.Equal(t, "team-a", got.ResourceName)
			assert.Equal(t, tt.wantApplied, got.Scope.Applied)
			assert.NotEmpty(t, got.Scope.Reason)
			assert.Equal(t, tt.wantStatus, got.Verdict.Status)
			assert.Contains(t, got.Verdict.Message, tt.wantMessage)
		})
	}
}

// TestHandle_TracingWithoutMatcher covers an engine built without a matcher: matchConstraints
// are not checked, and the scope must say so rather than claim they matched.
func TestHandle_TracingWithoutMatcher(t *testing.T) {
	policy := &v1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "generate-cm"},
		Spec: v1beta1.GeneratingPolicySpec{
			Generation: []v1beta1.Generation{{Expression: `generator.Apply(object.metadata.name, [{"apiVersion": dyn("v1"), "kind": dyn("ConfigMap"), "metadata": dyn({"name": "cm", "namespace": object.metadata.name})}])`}},
		},
	}
	compiled, errs := compiler.NewCompilerWithTrace(true).Compile(policy, nil)
	require.Nil(t, errs)
	trigger := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]any{"name": "team-a"}}}
	req := engine.Request(
		libs.NewFakeContextProvider(),
		schema.GroupVersionKind{Version: "v1", Kind: "Namespace"},
		schema.GroupVersionResource{Version: "v1", Resource: "namespaces"},
		"", "team-a", "", admissionv1.Create, authenticationv1.UserInfo{}, trigger, nil, false, nil,
	)
	resp, err := NewEngine(func(string) *corev1.Namespace { return nil }, nil).Handle(req, Policy{Policy: policy, CompiledPolicy: compiled}, false)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	got := resp.Policies[0].Trace
	require.NotNil(t, got)
	assert.True(t, got.Scope.Applied)
	assert.Equal(t, "evaluated without a matcher, so matchConstraints were not checked here", got.Scope.Reason)
	assert.Equal(t, trace.VerdictPass, got.Verdict.Status)
}
