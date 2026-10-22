package webhook

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestReferencesNamespaceObject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		expression string
		want       bool
	}{
		{
			name:       "field access",
			expression: "'some-label' in namespaceObject.metadata.labels",
			want:       true,
		},
		{
			name:       "has macro",
			expression: "has(namespaceObject.metadata.annotations)",
			want:       true,
		},
		{
			name:       "nested in logical expression",
			expression: "request.operation == 'CREATE' && namespaceObject.metadata.name != 'default'",
			want:       true,
		},
		{
			name:       "no reference",
			expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')",
			want:       false,
		},
		{
			name:       "similar identifier is not a match",
			expression: "object.metadata.name != 'namespaceObject'",
			want:       false,
		},
		{
			name:       "optional field selection syntax without reference",
			expression: `object.metadata.?labels["opt-in"].orValue("") == "true"`,
			want:       false,
		},
		{
			name:       "optional field selection syntax with reference",
			expression: `namespaceObject.metadata.?labels["opt-in"].orValue("") == "true"`,
			want:       true,
		},
		{
			name:       "unparsable expression is conservatively kept on the Kyverno side",
			expression: "namespaceObject.metadata.",
			want:       true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, referencesNamespaceObject(tt.expression))
		})
	}
}

func TestWebhookMatchConditions(t *testing.T) {
	t.Parallel()
	conditions := []admissionregistrationv1.MatchCondition{
		{Name: "safe", Expression: "request.operation == 'CREATE'"},
		{Name: "ns-label", Expression: "'some-label' in namespaceObject.metadata.labels"},
		{Name: "also-safe", Expression: "object.metadata.name != 'skip-me'"},
	}
	filtered := webhookMatchConditions(conditions)
	assert.Equal(t, []admissionregistrationv1.MatchCondition{
		{Name: "safe", Expression: "request.operation == 'CREATE'"},
		{Name: "also-safe", Expression: "object.metadata.name != 'skip-me'"},
	}, filtered)

	assert.Nil(t, webhookMatchConditions(nil))
	assert.Nil(t, webhookMatchConditions([]admissionregistrationv1.MatchCondition{
		{Name: "ns-label", Expression: "namespaceObject.metadata.name == 'prod'"},
	}))
}

func TestBuildWebhookRules_NamespaceObjectMatchConditionsNotOffloaded(t *testing.T) {
	matchConstraints := &admissionregistrationv1.MatchResources{
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
	}
	tests := []struct {
		name                    string
		matchConditions         []admissionregistrationv1.MatchCondition
		expectedMatchConditions []admissionregistrationv1.MatchCondition
	}{
		{
			name: "namespaceObject condition is kept out of the webhook configuration",
			matchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "ns-has-label", Expression: "'some-label' in namespaceObject.metadata.labels"},
			},
			expectedMatchConditions: nil,
		},
		{
			name: "only webhook-safe conditions are offloaded",
			matchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "ns-has-label", Expression: "'some-label' in namespaceObject.metadata.labels"},
				{Name: "exclude-leases", Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')"},
			},
			expectedMatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "exclude-leases", Expression: "!(request.resource.group == 'coordination.k8s.io' && request.resource.resource == 'leases')"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpol := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "check-ns-label",
				},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					FailurePolicy:    ptr.To(admissionregistrationv1.Fail),
					MatchConstraints: matchConstraints,
					MatchConditions:  tt.matchConditions,
				},
			}
			expressionCache := NewExpressionCache()
			expressionCache.AddPolicyExpressions(vpol.GetMatchConditions())
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"",
				config.ValidatingPolicyWebhookName,
				"/vpol",
				0,
				nil,
				[]engineapi.GenericPolicy{engineapi.NewValidatingPolicy(vpol)},
				expressionCache,
			)
			// the policy must stay fine-grained: it keeps its own webhook, only the
			// namespaceObject match conditions are left to the Kyverno engine
			assert.Len(t, webhooks, 1)
			assert.Equal(t, config.ValidatingPolicyWebhookName+"-fail-finegrained-check-ns-label", webhooks[0].Name)
			assert.Equal(t, tt.expectedMatchConditions, webhooks[0].MatchConditions)
		})
	}
}
