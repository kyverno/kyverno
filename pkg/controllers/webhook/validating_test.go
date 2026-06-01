package webhook

import (
	"fmt"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBuildWebhookRules_ValidatingPolicy(t *testing.T) {
	tests := []struct {
		name             string
		vpols            []*policiesv1beta1.ValidatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Single Ignore Policy",
			vpols: []*policiesv1beta1.ValidatingPolicy{
				{
					Spec: policiesv1beta1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"environment": "staging",
								},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ValidatingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Multiple Basic Policies with Different Selectors",
			vpols: []*policiesv1beta1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy-a",
					},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"env": "prod"},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "a"},
							},
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "policy-b",
					},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"env": "staging"},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "b"},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ValidatingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Single Fail Policy",
			vpols: []*policiesv1beta1.ValidatingPolicy{
				{
					Spec: policiesv1beta1.ValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Fail),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"environment": "staging",
								},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ValidatingPolicyWebhookName + "-fail",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Fail),
				},
			},
		},
		{
			name: "Fine-Grained Ignore Policy",
			vpols: []*policiesv1beta1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-ignore",
					},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						WebhookConfiguration: &policiesv1beta1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(30)),
						},
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"environment": "staging",
								},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
							},
							MatchPolicy: ptr.To(admissionregistrationv1.Exact),
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-ignore-finegrained-test-fine-grained-ignore",
					ClientConfig: newClientConfig("", 0, nil, "/vpol/test-fine-grained-ignore"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"environment": "staging",
						},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					FailurePolicy:  ptr.To(admissionregistrationv1.Ignore),
					TimeoutSeconds: ptr.To(int32(30)),
					MatchPolicy:    ptr.To(admissionregistrationv1.Exact),
				},
			},
		},
		{
			name: "Fine-Grained Fail Policy",
			vpols: []*policiesv1beta1.ValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-fine-grained-fail",
					},
					Spec: policiesv1beta1.ValidatingPolicySpec{
						WebhookConfiguration: &policiesv1beta1.WebhookConfiguration{
							TimeoutSeconds: ptr.To(int32(20)),
						},
						FailurePolicy: ptr.To(admissionregistrationv1.Fail),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"environment": "staging",
								},
							},
							ObjectSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "test",
								},
							},
							MatchPolicy: ptr.To(admissionregistrationv1.Exact),
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{"*"},
											APIVersions: []string{"*"},
											Resources:   []string{"*"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "exclude-leases",
								Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')",
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ValidatingPolicyWebhookName + "-fail-finegrained-test-fine-grained-fail",
					ClientConfig: newClientConfig("", 0, nil, "/vpol/test-fine-grained-fail"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"*"},
								APIVersions: []string{"*"},
								Resources:   []string{"*"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"environment": "staging",
						},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					FailurePolicy:  ptr.To(admissionregistrationv1.Fail),
					TimeoutSeconds: ptr.To(int32(20)),
					MatchPolicy:    ptr.To(admissionregistrationv1.Exact),
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "exclude-leases",
							Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionCache := NewExpressionCache()
			var vpols []engineapi.GenericPolicy
			for _, vpol := range tt.vpols {
				vpols = append(vpols, engineapi.NewValidatingPolicy(vpol))
				expressionCache.AddPolicyExpressions(vpol.GetMatchConditions())
			}
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.ValidatingPolicyWebhookName,
				"/vpol",
				0,
				nil,
				vpols,
				expressionCache,
			)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks))
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules))

				if expect.MatchConditions != nil {
					assert.Equal(t, expect.MatchConditions, webhooks[i].MatchConditions)
				}
				if expect.MatchPolicy != nil {
					assert.Equal(t, expect.MatchPolicy, webhooks[i].MatchPolicy)
				}
				if expect.TimeoutSeconds != nil {
					assert.Equal(t, expect.TimeoutSeconds, webhooks[i].TimeoutSeconds)
				}
				if expect.ClientConfig.Service != nil {
					assert.Equal(t, *webhooks[i].ClientConfig.Service.Path, *expect.ClientConfig.Service.Path)
				}
				assert.Equal(t, expect.NamespaceSelector, webhooks[i].NamespaceSelector)
				assert.Equal(t, expect.ObjectSelector, webhooks[i].ObjectSelector)
			}
		})
	}
}

func TestBuildWebhookRules_BasicWithGlobalSelectors(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"webhooks": `{
				"namespaceSelector": {
					"matchExpressions": [
						{"key": "kubernetes.io/metadata.name", "operator": "NotIn", "values": ["kyverno"]}
					]
				},
				"objectSelector": {
					"matchLabels": {"global": "true"}
				}
			}`,
		},
	})

	vpols := []engineapi.GenericPolicy{
		engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-a"},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "prod"},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "a"},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups: []string{""}, APIVersions: []string{"v1"},
									Resources: []string{"pods"},
								},
							},
						},
					},
				},
			},
		}),
		engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "policy-b"},
			Spec: policiesv1beta1.ValidatingPolicySpec{
				FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
				MatchConstraints: &admissionregistrationv1.MatchResources{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"env": "staging"},
					},
					ObjectSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "b"},
					},
					ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
						{
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule: admissionregistrationv1.Rule{
									APIGroups: []string{"apps"}, APIVersions: []string{"v1"},
									Resources: []string{"deployments"},
								},
							},
						},
					},
				},
			},
		}),
	}

	webhooks := buildWebhookRules(cfg, "", config.ValidatingPolicyWebhookName, "/vpol", 0, nil, vpols, NewExpressionCache())
	assert.Equal(t, 1, len(webhooks))
	assert.Equal(t, config.ValidatingPolicyWebhookName+"-ignore", webhooks[0].Name)
	assert.Equal(t, &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "kubernetes.io/metadata.name", Operator: metav1.LabelSelectorOpNotIn, Values: []string{"kyverno"}},
		},
	}, webhooks[0].NamespaceSelector)
	assert.Equal(t, &metav1.LabelSelector{
		MatchLabels: map[string]string{"global": "true"},
	}, webhooks[0].ObjectSelector)
}

func TestBuildWebhookRules_ImageValidatingPolicy(t *testing.T) {
	tests := []struct {
		name             string
		ivpols           []*policiesv1beta1.ImageValidatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Autogen Single Ignore Policy",
			ivpols: []*policiesv1beta1.ImageValidatingPolicy{
				{
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.ImageValidatingPolicyValidateWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"jobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"cronjobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Autogen Fine-grained Ignore Policy",
			ivpols: []*policiesv1beta1.ImageValidatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ivpol-sample",
						Annotations: map[string]string{
							kyverno.AnnotationAutogenControllers: "deployments,jobs,cronjobs",
						},
					},
					Spec: policiesv1beta1.ImageValidatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
											Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
										},
									},
								},
							},
						},
						MatchConditions: []admissionregistrationv1.MatchCondition{
							{
								Name:       "check-prod-label",
								Expression: "has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true'",
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name:         config.ImageValidatingPolicyValidateWebhookName + "-ignore-finegrained-ivpol-sample",
					ClientConfig: newClientConfig("", 0, nil, "/ivpol/validate/ivpol-sample"),
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"jobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"batch"},
								APIVersions: []string{"v1"},
								Resources:   []string{"cronjobs"},
								Scope:       ptr.To(admissionregistrationv1.ScopeType("*")),
							},
						},
					},
					FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
					MatchConditions: []admissionregistrationv1.MatchCondition{
						{
							Name:       "autogen-check-prod-label",
							Expression: "!((object.apiVersion == 'v1' && object.kind =='Pod')) || (has(object.metadata.labels) && has(object.metadata.labels.prod) && object.metadata.labels.prod == 'true')",
						},
						{
							Name:       "autogen-cronjobs-check-prod-label",
							Expression: "!((object.apiVersion == 'batch/v1' && object.kind =='CronJob')) || (has(object.spec.jobTemplate.spec.template.metadata.labels) && has(object.spec.jobTemplate.spec.template.metadata.labels.prod) && object.spec.jobTemplate.spec.template.metadata.labels.prod == 'true')",
						},
						{
							Name:       "autogen-defaults-check-prod-label",
							Expression: "!((object.apiVersion == 'apps/v1' && object.kind =='DaemonSet') || (object.apiVersion == 'apps/v1' && object.kind =='Deployment') || (object.apiVersion == 'apps/v1' && object.kind =='ReplicaSet') || (object.apiVersion == 'apps/v1' && object.kind =='StatefulSet') || (object.apiVersion == 'batch/v1' && object.kind =='Job')) || (has(object.spec.template.metadata.labels) && has(object.spec.template.metadata.labels.prod) && object.spec.template.metadata.labels.prod == 'true')",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionCache := NewExpressionCache()
			var ivpols []engineapi.GenericPolicy
			for _, ivpol := range tt.ivpols {
				ivpols = append(ivpols, engineapi.NewImageValidatingPolicy(ivpol))
				expressionCache.AddPolicyExpressions(ivpol.GetMatchConditions())
			}
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.ImageValidatingPolicyValidateWebhookName,
				"/ivpol/validate",
				0,
				nil,
				ivpols,
				expressionCache,
			)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks), tt.name)
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules), fmt.Sprintf("expected: %v,\n got: %v", expect.Rules, webhooks[i].Rules))

				if expect.MatchConditions != nil {
					assert.Equal(t, expect.MatchConditions, webhooks[i].MatchConditions)
				}
				if expect.MatchPolicy != nil {
					assert.Equal(t, expect.MatchPolicy, webhooks[i].MatchPolicy)
				}
				if expect.TimeoutSeconds != nil {
					assert.Equal(t, expect.TimeoutSeconds, webhooks[i].TimeoutSeconds)
				}
				if expect.ClientConfig.Service != nil {
					assert.Equal(t, *webhooks[i].ClientConfig.Service.Path, *expect.ClientConfig.Service.Path)
				}
			}
		})
	}
}

func TestBuildWebhookRules_MutatingPolicy(t *testing.T) {
	tests := []struct {
		name             string
		mpols            []*policiesv1beta1.MutatingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Single Basic Mutating Policy with namespaceSelector should not propagate to shared webhook",
			mpols: []*policiesv1beta1.MutatingPolicy{
				{
					Spec: policiesv1beta1.MutatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "a"},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.MutatingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Multiple Basic Mutating Policies with Different namespaceSelectors",
			mpols: []*policiesv1beta1.MutatingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mpol-a",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "a"},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"pods"},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "mpol-b",
					},
					Spec: policiesv1beta1.MutatingPolicySpec{
						FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "b"},
							},
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
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.MutatingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionCache := NewExpressionCache()
			var mpols []engineapi.GenericPolicy
			for _, mpol := range tt.mpols {
				mpols = append(mpols, engineapi.NewMutatingPolicy(mpol))
				expressionCache.AddPolicyExpressions(mpol.GetMatchConditions())
			}
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.MutatingPolicyWebhookName,
				"/mpol",
				0,
				nil,
				mpols,
				expressionCache,
			)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks))
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules))
				assert.Equal(t, expect.NamespaceSelector, webhooks[i].NamespaceSelector)
				assert.Equal(t, expect.ObjectSelector, webhooks[i].ObjectSelector)
			}
		})
	}
}

func TestBuildWebhookRules_GeneratingPolicy(t *testing.T) {
	tests := []struct {
		name             string
		gpols            []*policiesv1beta1.GeneratingPolicy
		expectedWebhooks []admissionregistrationv1.ValidatingWebhook
	}{
		{
			name: "Single Basic Generating Policy with namespaceSelector should not propagate to shared webhook",
			gpols: []*policiesv1beta1.GeneratingPolicy{
				{
					Spec: policiesv1beta1.GeneratingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "a"},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"configmaps"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.GeneratingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Connect,
								admissionregistrationv1.Create,
								admissionregistrationv1.Delete,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
		{
			name: "Multiple Basic Generating Policies with Different namespaceSelectors",
			gpols: []*policiesv1beta1.GeneratingPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpol-a",
					},
					Spec: policiesv1beta1.GeneratingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "a"},
							},
							ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
								{
									RuleWithOperations: admissionregistrationv1.RuleWithOperations{
										Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
										Rule: admissionregistrationv1.Rule{
											APIGroups:   []string{""},
											APIVersions: []string{"v1"},
											Resources:   []string{"configmaps"},
										},
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "gpol-b",
					},
					Spec: policiesv1beta1.GeneratingPolicySpec{
						MatchConstraints: &admissionregistrationv1.MatchResources{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"team": "b"},
							},
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
			expectedWebhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: config.GeneratingPolicyWebhookName + "-ignore",
					Rules: []admissionregistrationv1.RuleWithOperations{
						{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Connect,
								admissionregistrationv1.Create,
								admissionregistrationv1.Delete,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"configmaps"},
							},
						},
						{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Connect,
								admissionregistrationv1.Create,
								admissionregistrationv1.Delete,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
					NamespaceSelector: nil,
					ObjectSelector:    nil,
					FailurePolicy:     ptr.To(admissionregistrationv1.Ignore),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expressionCache := NewExpressionCache()
			var gpols []engineapi.GenericPolicy
			for _, gpol := range tt.gpols {
				gpols = append(gpols, engineapi.NewGeneratingPolicy(gpol))
				expressionCache.AddPolicyExpressions(gpol.GetMatchConditions())
			}
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.GeneratingPolicyWebhookName,
				"/gpol",
				0,
				nil,
				gpols,
				expressionCache,
			)
			assert.Equal(t, len(tt.expectedWebhooks), len(webhooks))
			for i, expect := range tt.expectedWebhooks {
				assert.Equal(t, expect.Name, webhooks[i].Name)
				assert.Equal(t, expect.FailurePolicy, webhooks[i].FailurePolicy)
				assert.Equal(t, len(expect.Rules), len(webhooks[i].Rules))
				assert.Equal(t, expect.NamespaceSelector, webhooks[i].NamespaceSelector)
				assert.Equal(t, expect.ObjectSelector, webhooks[i].ObjectSelector)
			}
		})
	}
}
