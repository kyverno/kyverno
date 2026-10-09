package webhook

import (
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func (c *controller) buildGenerationLabelProtectionWebhook(caBundle []byte) admissionregistrationv1.ValidatingWebhook {
	timeout := capTimeout(c.defaultTimeout)
	return admissionregistrationv1.ValidatingWebhook{
		Name:         config.GenerationLabelProtectionWebhookName,
		ClientConfig: newClientConfig(c.server, c.servicePort, caBundle, config.GenerationLabelProtectionWebhookServicePath),
		Rules: []admissionregistrationv1.RuleWithOperations{{
			Rule: admissionregistrationv1.Rule{
				APIGroups:   []string{"*"},
				APIVersions: []string{"*"},
				Resources:   []string{"*/*"},
			},
			Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
		}},
		FailurePolicy:           ptr.To(admissionregistrationv1.Fail),
		SideEffects:             ptr.To(admissionregistrationv1.SideEffectClassNone),
		AdmissionReviewVersions: []string{"v1"},
		TimeoutSeconds:          &timeout,
		MatchPolicy:             ptr.To(admissionregistrationv1.Equivalent),
		// Every controller-owned downstream has a policy association. Kubernetes
		// checks both old and new objects, so removing it cannot bypass protection.
		// Object selectors also work on servers predating webhook match conditions.
		ObjectSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key: common.GeneratePolicyLabel, Operator: metav1.LabelSelectorOpExists,
		}}},
	}
}

// Policy routing may be user-managed when autoUpdateWebhooks is disabled. Only
// the reserved metadata webhook is reconciled in that mode, including upgrades.
func setGenerationLabelProtectionWebhook(configuration *admissionregistrationv1.ValidatingWebhookConfiguration, protection admissionregistrationv1.ValidatingWebhook) {
	for i := range configuration.Webhooks {
		if configuration.Webhooks[i].Name == config.GenerationLabelProtectionWebhookName {
			configuration.Webhooks[i] = protection
			return
		}
	}
	configuration.Webhooks = append(configuration.Webhooks, protection)
}
