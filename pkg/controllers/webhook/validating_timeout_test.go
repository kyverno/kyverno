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

// vpolForTimeout builds a ValidatingPolicy without any match conditions, so it is
// grouped into a basic webhook unless it carries a per-policy timeout (which makes
// it fine-grained).
func vpolForTimeout(name string, timeout *int32) engineapi.GenericPolicy {
	spec := policiesv1beta1.ValidatingPolicySpec{
		FailurePolicy: ptr.To(admissionregistrationv1.Ignore),
		MatchConstraints: &admissionregistrationv1.MatchResources{
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
	}
	if timeout != nil {
		spec.WebhookConfiguration = &policiesv1beta1.WebhookConfiguration{TimeoutSeconds: timeout}
	}
	return engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       spec,
	})
}

// TestBuildWebhookRulesAppliesDefaultTimeout covers https://github.com/kyverno/kyverno/issues/17733:
// the admission controller's --webhookTimeout flag (the defaultTimeout parameter) must be
// applied to ValidatingPolicy webhooks, which previously always fell back to the API
// server default of 10s.
func TestBuildWebhookRulesAppliesDefaultTimeout(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	tests := []struct {
		name           string
		defaultTimeout int32
		policyTimeout  *int32
		expected       int32
	}{
		{name: "flag only applies to basic webhooks", defaultTimeout: 1, expected: 1},
		{name: "default flag keeps the ten second default", defaultTimeout: 10, expected: 10},
		{name: "larger flag than policy timeout wins", defaultTimeout: 30, policyTimeout: ptr.To[int32](5), expected: 30},
		{name: "larger policy timeout than flag wins", defaultTimeout: 1, policyTimeout: ptr.To[int32](20), expected: 20},
		{name: "timeout is capped at thirty seconds", defaultTimeout: 60, expected: 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webhooks := buildWebhookRules(
				cfg,
				"",
				config.ValidatingPolicyWebhookName,
				"/vpol",
				0,
				tt.defaultTimeout,
				nil,
				[]engineapi.GenericPolicy{vpolForTimeout("test-webhook-timeout", tt.policyTimeout)},
				NewExpressionCache(),
			)
			assert.NotEmpty(t, webhooks)
			for _, webhook := range webhooks {
				assert.Equal(t, tt.expected, *webhook.TimeoutSeconds, "webhook %s", webhook.Name)
			}
		})
	}
}

// TestBuildWebhookRulesBasicGroupMaxTimeout ensures a basic webhook aggregating several
// policies uses the maximum of the flag default and any per-policy timeout in the group.
func TestBuildWebhookRulesBasicGroupMaxTimeout(t *testing.T) {
	cfg := config.NewDefaultConfiguration(false)
	webhooks := buildWebhookRules(
		cfg,
		"",
		config.ValidatingPolicyWebhookName,
		"/vpol",
		0,
		20,
		nil,
		[]engineapi.GenericPolicy{vpolForTimeout("one", nil), vpolForTimeout("two", nil)},
		NewExpressionCache(),
	)
	// both policies resolve to the same selectors, so they share one webhook
	assert.NotEmpty(t, webhooks)
	for _, webhook := range webhooks {
		assert.Equal(t, int32(20), *webhook.TimeoutSeconds, "webhook %s", webhook.Name)
	}
}
