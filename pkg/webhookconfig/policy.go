package webhookconfig

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) contructPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyValidatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			generateWebhook(
				config.PolicyValidatingWebhookName,
				config.PolicyValidatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				"clusterpolicies/*",
				"kyverno.io",
				"v1alpha1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) contructDebugPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.PolicyValidatingWebhookServicePath)
	glog.V(4).Infof("Debug PolicyValidatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyValidatingWebhookConfigurationDebugName,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			generateDebugWebhook(
				config.PolicyValidatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				"clusterpolicies/*",
				"kyverno.io",
				"v1alpha1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) contructPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyMutatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			generateWebhook(
				config.PolicyMutatingWebhookName,
				config.PolicyMutatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				"clusterpolicies/*",
				"kyverno.io",
				"v1alpha1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}
func (wrc *WebhookRegistrationClient) contructDebugPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.PolicyMutatingWebhookServicePath)
	glog.V(4).Infof("Debug PolicyMutatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.PolicyMutatingWebhookConfigurationDebugName,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			generateDebugWebhook(
				config.PolicyMutatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				"clusterpolicies/*",
				"kyverno.io",
				"v1alpha1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}
