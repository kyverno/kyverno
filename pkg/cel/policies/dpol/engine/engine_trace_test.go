package engine

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TestHandle_Tracing drives Handle with a real matcher for each outcome and checks the trace the
// engine attaches: the header, SCOPE (which must never show the placeholder CREATE operation the
// engine matches with), the verdict, and that tracing off leaves the response exactly as before.
func TestHandle_Tracing(t *testing.T) {
	dpol := &policiesv1beta1.DeletingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "delete-temp-pods"},
		Spec: policiesv1beta1.DeletingPolicySpec{
			Schedule: "*/5 * * * *",
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						// CREATE only, to prove the deletion scan ignores operations
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"},
						},
					},
				}},
			},
			Conditions: []admissionregistrationv1.MatchCondition{
				{Name: "is-temporary", Expression: "object.metadata.labels.tier == 'temp'"},
			},
		},
	}
	object := func(kind string, labels map[string]any) unstructured.Unstructured {
		u := unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": kind, "metadata": map[string]any{"name": "res", "namespace": "prod"}}}
		if labels != nil {
			u.SetLabels(map[string]string{})
			_ = unstructured.SetNestedMap(u.Object, labels, "metadata", "labels")
		}
		return u
	}
	tests := []struct {
		name          string
		resource      unstructured.Unstructured
		wantErr       bool
		wantMatch     bool
		wantMatched   bool
		wantApplied   bool
		wantScope     string
		wantStatus    string
		wantCondition bool // the condition appears in the trace
	}{{
		name:          "conditions hold",
		resource:      object("Pod", map[string]any{"tier": "temp"}),
		wantMatch:     true,
		wantMatched:   true,
		wantApplied:   true,
		wantScope:     "matched kind Pod, namespace prod (deletion scan, so operations are not checked)",
		wantStatus:    trace.VerdictPass,
		wantCondition: true,
	}, {
		name:          "condition false",
		resource:      object("Pod", map[string]any{"tier": "web"}),
		wantMatched:   true,
		wantApplied:   true,
		wantScope:     "matched kind Pod, namespace prod",
		wantStatus:    trace.VerdictFail,
		wantCondition: true,
	}, {
		name:          "condition errors",
		resource:      object("Pod", nil),
		wantErr:       true,
		wantApplied:   true,
		wantScope:     "matched kind Pod, namespace prod",
		wantStatus:    trace.VerdictError,
		wantCondition: true,
	}, {
		name:       "matchConstraints do not cover the resource",
		resource:   object("ConfigMap", nil),
		wantScope:  "kind ConfigMap, namespace prod is not covered by the policy's resourceRules",
		wantStatus: trace.VerdictSkip,
	}, {
		name:          "JSON payload",
		resource:      unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"labels": map[string]any{"tier": "temp"}}}},
		wantMatch:     true,
		wantMatched:   true,
		wantApplied:   true,
		wantScope:     "evaluated against a JSON payload, so no matchConstraints apply",
		wantStatus:    trace.VerdictPass,
		wantCondition: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := func(traced bool) (EngineResponse, error) {
				compiled, errs := compiler.NewCompilerWithTrace(traced).Compile(dpol, nil)
				require.Empty(t, errs)
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "Pod"}, meta.RESTScopeNamespace)
				mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
				eng := NewEngine(func(ns string) *corev1.Namespace {
					return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
				}, mapper, &libs.FakeContextProvider{}, matching.NewMatcher())
				return eng.Handle(context.Background(), Policy{Policy: dpol, CompiledPolicy: compiled}, tt.resource)
			}

			untraced, untracedErr := handle(false)
			traced, tracedErr := handle(true)

			// tracing must not change the outcome
			assert.Equal(t, tt.wantErr, untracedErr != nil)
			assert.Equal(t, tt.wantErr, tracedErr != nil)
			if tt.wantErr {
				assert.Equal(t, untracedErr.Error(), tracedErr.Error())
			}
			assert.Equal(t, tt.wantMatch, untraced.Match)
			assert.Equal(t, tt.wantMatch, traced.Match)
			assert.Equal(t, tt.wantMatched, untraced.PolicyMatched)
			assert.Equal(t, tt.wantMatched, traced.PolicyMatched)
			assert.Nil(t, untraced.Trace, "no trace when compiled without tracing")

			got := traced.Trace
			require.NotNil(t, got)
			assert.Equal(t, "delete-temp-pods", got.PolicyName)
			assert.Equal(t, "DeletingPolicy", got.PolicyKind)
			assert.Equal(t, tt.wantApplied, got.Scope.Applied)
			assert.Contains(t, got.Scope.Reason, tt.wantScope)
			assert.NotContains(t, got.Scope.Reason, "operation CREATE", "a deletion scan has no admission operation")
			assert.Equal(t, tt.wantStatus, got.Verdict.Status)
			if tt.wantCondition {
				require.Len(t, got.Match, 1)
				assert.Equal(t, "is-temporary", got.Match[0].Name)
			} else {
				assert.Empty(t, got.Match)
			}
		})
	}
}
