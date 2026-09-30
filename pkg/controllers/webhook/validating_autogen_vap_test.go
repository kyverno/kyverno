package webhook

import (
	"slices"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func podValidatingPolicy(vapEnabled, generated bool, matchConditions []admissionregistrationv1.MatchCondition) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-pods"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: matchConditions,
			Validations:     []admissionregistrationv1.Validation{{Expression: "true"}},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				ValidatingAdmissionPolicy: &policiesv1beta1.VapGenerationConfiguration{Enabled: ptr.To(vapEnabled)},
			},
		},
		Status: policiesv1beta1.ValidatingPolicyStatus{Generated: generated},
	}
}

func webhookResources(webhooks []admissionregistrationv1.ValidatingWebhook) []string {
	var resources []string
	for _, webhook := range webhooks {
		for _, rule := range webhook.Rules {
			resources = append(resources, rule.Resources...)
		}
	}
	return resources
}

func TestBuildWebhookRules_AutogenCoveredByVAP(t *testing.T) {
	fineGrained := []admissionregistrationv1.MatchCondition{{Name: "always-true", Expression: "true"}}
	tests := []struct {
		name            string
		vapEnabled      bool
		generated       bool
		matchConditions []admissionregistrationv1.MatchCondition
		wantAutogen     bool
	}{
		{name: "basic: vap disabled", vapEnabled: false, generated: false, wantAutogen: true},
		{name: "basic: vap enabled but not generated yet", vapEnabled: true, generated: false, wantAutogen: true},
		{name: "basic: vap enabled and generated", vapEnabled: true, generated: true, wantAutogen: false},
		{name: "basic: vap disabled with stale generated status", vapEnabled: false, generated: true, wantAutogen: true},
		{name: "fine grained: vap enabled but not generated yet", vapEnabled: true, generated: false, matchConditions: fineGrained, wantAutogen: true},
		{name: "fine grained: vap enabled and generated", vapEnabled: true, generated: true, matchConditions: fineGrained, wantAutogen: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := engineapi.NewValidatingPolicy(podValidatingPolicy(tt.vapEnabled, tt.generated, tt.matchConditions))
			cache := NewExpressionCache()
			cache.AddPolicyExpressions(extractGenericPolicy(policy).GetMatchConditions())
			webhooks := buildWebhookRules(
				config.NewDefaultConfiguration(false),
				"", config.ValidatingPolicyWebhookName, "/vpol",
				0, nil, []engineapi.GenericPolicy{policy}, cache,
			)
			resources := webhookResources(webhooks)
			assert.Contains(t, resources, "pods")
			assert.Equal(t, tt.wantAutogen, slices.Contains(resources, "deployments"))
			assert.Equal(t, tt.wantAutogen, slices.Contains(resources, "cronjobs"))
		})
	}
}

func TestAutogenCoveredByVAP(t *testing.T) {
	assert.False(t, autogenCoveredByVAP(nil))
	assert.False(t, autogenCoveredByVAP(&policiesv1beta1.NamespacedValidatingPolicy{}))
	assert.False(t, autogenCoveredByVAP(podValidatingPolicy(false, true, nil)))
	assert.False(t, autogenCoveredByVAP(podValidatingPolicy(true, false, nil)))
	assert.True(t, autogenCoveredByVAP(podValidatingPolicy(true, true, nil)))
}
