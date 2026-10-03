package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

// TestHandle_Tracing drives the admission path (Handle, with a real matcher) for each outcome
// a traced MutatingPolicy can reach, and checks the trace the engine attaches: the
// policy/resource header, SCOPE, the verdict, and that tracing off leaves Trace nil.
func TestHandle_Tracing(t *testing.T) {
	const addLabel = `Object{metadata: Object.metadata{labels: {"team": "platform"}}}`
	newPolicy := func(resource string, conditions []admissionregistrationv1.MatchCondition, mutation string) *policiesv1beta1.MutatingPolicy {
		return &policiesv1beta1.MutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"},
			Spec: policiesv1beta1.MutatingPolicySpec{
				// no autogen: a pods policy otherwise also yields Deployment/CronJob variants,
				// each returned as its own response, and this test is about one policy's trace
				AutogenConfiguration: &policiesv1beta1.MutatingPolicyAutogenConfiguration{
					PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{Controllers: []string{}},
				},
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
				Mutations: []admissionregistrationv1alpha1.Mutation{{
					PatchType:          admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{Expression: mutation},
				}},
			},
		}
	}
	exemptAll := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt-all", Namespace: "prod"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: "add-team-label", Kind: "MutatingPolicy"}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "always", Expression: "true"}},
		},
	}
	tests := []struct {
		name         string
		policy       *policiesv1beta1.MutatingPolicy
		exceptions   []*policiesv1beta1.PolicyException
		wantStatus   string
		wantApplied  bool
		wantMessage  string // checked with Contains when set
		wantMutation bool   // a MUTATIONS entry is recorded
		wantPatched  bool
	}{{
		name:         "mutation applies",
		policy:       newPolicy("pods", nil, addLabel),
		wantStatus:   trace.VerdictPass,
		wantApplied:  true,
		wantMutation: true,
		wantPatched:  true,
	}, {
		name:        "matchConstraints do not cover the resource",
		policy:      newPolicy("configmaps", nil, addLabel),
		wantStatus:  trace.VerdictSkip,
		wantApplied: false,
		wantMessage: "the policy does not apply to this resource",
	}, {
		name: "match condition excludes the resource",
		policy: newPolicy("pods", []admissionregistrationv1.MatchCondition{{
			Name: "not-prod", Expression: "object.metadata.namespace != 'prod'",
		}}, addLabel),
		wantStatus:  trace.VerdictSkip,
		wantApplied: true,
		wantMessage: "not-prod",
	}, {
		name:         "mutation errors",
		policy:       newPolicy("pods", nil, `Object{metadata: Object.metadata{labels: {"copied": object.metadata.labels.owner}}}`),
		wantStatus:   trace.VerdictError,
		wantApplied:  true,
		wantMutation: true,
	}, {
		name:        "exempted by a policy exception",
		policy:      newPolicy("pods", nil, addLabel),
		exceptions:  []*policiesv1beta1.PolicyException{exemptAll},
		wantStatus:  trace.VerdictSkip,
		wantApplied: true,
		wantMessage: "exempted by a policy exception",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handle := func(traced bool) EngineResponse {
				provider, err := NewProvider(compiler.NewCompiler(traced), []policiesv1beta1.MutatingPolicyLike{tt.policy}, tt.exceptions, libs.NewFakeContextProvider())
				require.NoError(t, err)
				eng := NewEngine(provider,
					func(ns string) *corev1.Namespace { return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}} },
					matching.NewMatcher(), &fakeTypeConverter{}, &libs.FakeContextProvider{},
				)
				pod := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"nginx","namespace":"prod"},"spec":{"containers":[{"name":"web","image":"nginx"}]}}`)
				dryRun := true
				resp, err := eng.Handle(ctx, engine.EngineRequest{Request: admissionv1.AdmissionRequest{
					Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
					Resource:  metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
					Namespace: "prod",
					Name:      "nginx",
					Operation: admissionv1.Create,
					Object:    runtime.RawExtension{Raw: pod},
					DryRun:    &dryRun,
				}}, predicate)
				require.NoError(t, err)
				require.Len(t, resp.Policies, 1)
				return resp
			}

			resp := handle(true)
			got := resp.Policies[0].Trace
			require.NotNil(t, got)
			assert.Equal(t, "add-team-label", got.PolicyName)
			assert.Equal(t, "MutatingPolicy", got.PolicyKind)
			assert.Equal(t, "Pod", got.ResourceKind)
			assert.Equal(t, "nginx", got.ResourceName)
			assert.Equal(t, "prod", got.ResourceNamespace)
			assert.Equal(t, tt.wantApplied, got.Scope.Applied)
			assert.NotEmpty(t, got.Scope.Reason)
			assert.Equal(t, tt.wantStatus, got.Verdict.Status)
			if tt.wantMessage != "" {
				assert.Contains(t, got.Verdict.Message, tt.wantMessage)
			}
			if tt.wantMutation {
				require.Len(t, got.Mutations, 1)
				assert.Equal(t, tt.wantStatus == trace.VerdictError, got.Mutations[0].Error != "")
			} else {
				assert.Empty(t, got.Mutations)
			}
			if tt.wantPatched {
				require.NotNil(t, resp.PatchedResource)
				assert.Equal(t, "platform", resp.PatchedResource.GetLabels()["team"])
			}

			untraced := handle(false)
			assert.Nil(t, untraced.Policies[0].Trace, "no trace when compiled without tracing")
			assert.Equal(t, len(resp.Policies[0].Rules), len(untraced.Policies[0].Rules))
			for i := range untraced.Policies[0].Rules {
				assert.Equal(t, untraced.Policies[0].Rules[i].Status(), resp.Policies[0].Rules[i].Status(), "tracing must not change the rule outcome")
			}
		})
	}
}

// TestEvaluate_MutateExistingScopeIsNotLabelledJSON covers the no-matcher path that the CLI's and
// the background controller's mutate-existing engines take. mpol has no JSON-payload mode, so the
// scope must describe a mutate-existing evaluation rather than reuse vpol's JSON-payload wording.
func TestEvaluate_MutateExistingScopeIsNotLabelledJSON(t *testing.T) {
	configMapRule := admissionregistrationv1.NamedRuleWithOperations{
		RuleWithOperations: admissionregistrationv1.RuleWithOperations{
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Update},
			Rule: admissionregistrationv1.Rule{
				APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"},
			},
		},
	}
	tests := []struct {
		name       string
		target     *policiesv1beta1.TargetMatchConstraints
		wantReason string
	}{{
		name: "explicit target",
		target: &policiesv1beta1.TargetMatchConstraints{
			MatchResources: admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{configMapRule},
			},
		},
		wantReason: "selected by the policy's targetMatchConstraints",
	}, {
		name:       "no explicit target",
		wantReason: "evaluated without a matcher (mutate-existing)",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mutateExisting := true
			mpol := &policiesv1beta1.MutatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "label-target"},
				Spec: policiesv1beta1.MutatingPolicySpec{
					EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
						MutateExistingConfiguration: &policiesv1beta1.MutateExistingConfiguration{Enabled: &mutateExisting},
					},
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{configMapRule},
					},
					TargetMatchConstraints: tt.target,
					Mutations: []admissionregistrationv1alpha1.Mutation{{
						PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
						ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
							Expression: `Object{metadata: Object.metadata{labels: {"mutated": "true"}}}`,
						},
					}},
				},
			}
			evaluate := func(traced bool) EngineResponse {
				provider, err := NewProvider(compiler.NewCompiler(traced), []policiesv1beta1.MutatingPolicyLike{mpol}, nil, libs.NewFakeContextProvider())
				require.NoError(t, err)
				target := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "target", "namespace": "default"},
				}}
				attr := admission.NewAttributesRecord(
					target, nil, schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
					"default", "target", schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
					"", admission.Update, nil, false, &user.DefaultInfo{},
				)
				request := admissionv1.AdmissionRequest{Operation: admissionv1.Update, Name: "target", Namespace: "default"}
				// nil matcher: the mutate-existing engine shape
				eng := NewEngine(provider, nsResolver, nil, &fakeTypeConverter{}, &libs.FakeContextProvider{})
				response, err := eng.Evaluate(ctx, attr, request, predicate)
				require.NoError(t, err)
				require.Len(t, response.Policies, 1)
				return response
			}

			traced := evaluate(true)
			got := traced.Policies[0].Trace
			require.NotNil(t, got)
			assert.True(t, got.Scope.Applied)
			assert.Contains(t, got.Scope.Reason, tt.wantReason)
			assert.NotContains(t, got.Scope.Reason, "JSON payload")
			if assert.NotNil(t, traced.PatchedResource) {
				assert.Equal(t, "true", traced.PatchedResource.GetLabels()["mutated"])
			}

			assert.Nil(t, evaluate(false).Policies[0].Trace, "no trace when compiled without tracing")
		})
	}
}
