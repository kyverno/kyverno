package webhookconfig

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *Register) contructPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyValidatingWebhookConfigurationName,
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(
				config.PolicyValidatingWebhookName,
				config.PolicyValidatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"clusterpolicies/*", "policies/*"},
				"kyverno.io",
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *Register) contructDebugPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	logger := wrc.log
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.PolicyValidatingWebhookServicePath)
	logger.V(4).Info("Debug PolicyValidatingWebhookConfig is registered with url ", "url", url)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyValidatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateDebugValidatingWebhook(
				config.PolicyValidatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"clusterpolicies/*", "policies/*"},
				"kyverno.io",
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *Register) contructPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyMutatingWebhookConfigurationName,
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(
				config.PolicyMutatingWebhookName,
				config.PolicyMutatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"clusterpolicies/*", "policies/*"},
				"kyverno.io",
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *Register) contructDebugPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	logger := wrc.log
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.PolicyMutatingWebhookServicePath)
	logger.V(4).Info("Debug PolicyMutatingWebhookConfig is registered with url ", "url", url)

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyMutatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(
				config.PolicyMutatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				[]string{"clusterpolicies/*", "policies/*"},
				"kyverno.io",
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}
