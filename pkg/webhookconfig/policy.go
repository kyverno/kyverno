package webhookconfig

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *Register) constructPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyValidatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(
				config.PolicyValidatingWebhookName,
				config.PolicyValidatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				admregapi.Rule{
					Resources:   []string{"clusterpolicies/*", "policies/*"},
					APIGroups:   []string{"kyverno.io"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
		},
	}
}

func (wrc *Register) constructDebugPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
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
				admregapi.Rule{
					Resources:   []string{"clusterpolicies/*", "policies/*"},
					APIGroups:   []string{"kyverno.io"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
		},
	}
}

func (wrc *Register) constructPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyMutatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(
				config.PolicyMutatingWebhookName,
				config.PolicyMutatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				admregapi.Rule{
					Resources:   []string{"clusterpolicies/*", "policies/*"},
					APIGroups:   []string{"kyverno.io"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
		},
	}
}

func (wrc *Register) constructDebugPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
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
				admregapi.Rule{
					Resources:   []string{"clusterpolicies/*", "policies/*"},
					APIGroups:   []string{"kyverno.io"},
					APIVersions: []string{"v1"},
				},
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
		},
	}
}
