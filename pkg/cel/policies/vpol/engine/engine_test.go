package engine

import (
	"context"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestHandle(t *testing.T) {
	nsResolver := func(_ string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
	}

	makeReq := func(kind, namespace, raw string) engine.EngineRequest {
		return engine.EngineRequest{
			Request: admissionv1.AdmissionRequest{
				Operation: admissionv1.Create,
				Kind:      metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: kind},
				Resource:  metav1.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
				Namespace: namespace,
				Name:      "nginx",
				Object:    apiruntime.RawExtension{Raw: []byte(raw)},
				RequestResource: &metav1.GroupVersionResource{
					Group: "apps", Version: "v1", Resource: "deployments",
				},
			},
			Context: libs.NewFakeContextProvider(),
		}
	}

	makeProvider := func(t *testing.T, policies []policiesv1beta1.ValidatingPolicyLike) ProviderFunc {
		t.Helper()
		compiledPolicies := make([]Policy, 0, len(policies))
		comp := vpolcompiler.NewCompiler()
		for _, p := range policies {
			compiled, errs := comp.Compile(p, nil)
			if len(errs) > 0 {
				t.Fatalf("failed to compile policy %s: %v", p.GetName(), errs.ToAggregate())
			}
			compiledPolicies = append(compiledPolicies, Policy{
				Policy:         p,
				CompiledPolicy: compiled,
				Actions:        sets.Set[admissionregistrationv1.ValidationAction]{admissionregistrationv1.Deny: sets.Empty{}},
			})
		}
		return ProviderFunc(func(context.Context) ([]Policy, error) {
			return compiledPolicies, nil
		})
	}

	tests := []struct {
		name           string
		policies       []policiesv1beta1.ValidatingPolicyLike
		requestObject  string
		kind           string
		matchNamespace string
		predicate      Predicate
		expectPolicies int
		expectAllowed  bool
		expectReason   string
		expectWarnings []string
	}{
		{
			name: "Successful validation - object matches requirements",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-labels"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels.env)", Message: "Deployment must have 'env' label"},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default","labels":{"env":"production"}}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectAllowed:  true,
		},
		{
			name: "Validation fails - missing required label",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-labels"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels) && has(object.metadata.labels.env)", Message: "Deployment must have 'env' label"},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectAllowed:  false,
			expectReason:   "Deployment must have 'env' label",
		},
		{
			name: "predicate returns false - policy skipped",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "skip-policy"},
					Spec:       policiesv1beta1.ValidatingPolicySpec{},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return false },
			expectPolicies: 0,
			expectAllowed:  true,
		},
		{
			name: "no validation specified - allows by default",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "no-validation"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectAllowed:  true,
		},
		{
			name: "Multiple policies - all pass",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-env-label"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels.env)", Message: "Must have env label"},
						},
					},
				},
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-team-label"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels.team)", Message: "Must have team label"},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default","labels":{"env":"production","team":"platform"}}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 2,
			expectAllowed:  true,
		},
		{
			name: "Multiple policies - one fails",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-env-label"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels.env)", Message: "Must have env label"},
						},
					},
				},
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "require-team-label"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "has(object.metadata.labels.team)", Message: "Must have team label"},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default","labels":{"env":"production"}}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 2,
			expectAllowed:  false,
			expectReason:   "Must have team label",
		},
		{
			name: "Validation with warning message expression",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "warn-missing-label"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{
								Expression:        "has(object.metadata.labels) && has(object.metadata.labels.env)",
								Message:           "Deployment should have 'owner' label",
								MessageExpression: `"Warning: " + object.metadata.name + " is missing owner label"`,
							},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectAllowed:  true,
			expectWarnings: []string{"Warning: nginx is missing owner label"},
		},
		{
			name: "Complex validation - check replica count",
			policies: []policiesv1beta1.ValidatingPolicyLike{
				&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "max-replicas"},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"apps"},
											APIVersions: []string{"v1"},
											Resources:   []string{"deployments"},
										},
									},
								},
							},
						},
						Validations: []admissionregistrationv1.Validation{
							{Expression: "object.spec.replicas <= 10", Message: "Deployment cannot have more than 10 replicas"},
						},
					},
				},
			},
			requestObject:  `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default"},"spec":{"replicas":15}}`,
			kind:           "Deployment",
			matchNamespace: "default",
			predicate:      func(p policiesv1beta1.ValidatingPolicyLike) bool { return true },
			expectPolicies: 1,
			expectAllowed:  false,
			expectReason:   "Deployment cannot have more than 10 replicas",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			provider := makeProvider(t, tc.policies)
			eng := NewEngine(provider, nsResolver, matching.NewMatcher())
			req := makeReq(tc.kind, tc.matchNamespace, tc.requestObject)

			resp, err := eng.Handle(context.Background(), req, tc.predicate)
			assert.NoError(t, err)
			assert.Len(t, resp.Policies, tc.expectPolicies)

			allowed := true
			reasons := []string{}
			warnings := []string{}

			for _, pol := range resp.Policies {
				for _, rule := range pol.Rules {
					msg := rule.Message()
					if strings.HasPrefix(msg, "Warning:") {
						warnings = append(warnings, msg)
						continue
					}
					if rule.Status() == engineapi.RuleStatusFail || rule.Status() == engineapi.RuleStatusError {
						allowed = false
						if msg != "" {
							reasons = append(reasons, msg)
						}
					}
				}
			}

			assert.Equal(t, tc.expectAllowed, allowed)
			if tc.expectReason != "" {
				assert.Contains(t, strings.Join(reasons, " | "), tc.expectReason)
			}
			if len(tc.expectWarnings) > 0 {
				assert.ElementsMatch(t, tc.expectWarnings, warnings)
			}
		})
	}
}
