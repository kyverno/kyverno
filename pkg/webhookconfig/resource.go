package webhookconfig

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) contructDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.MutatingWebhookServicePath)
	glog.V(3).Infof("Debug MutatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.MutatingWebhookConfigurationDebug,
			Labels: config.KubePolicyAppLabels,
		},
		Webhooks: []admregapi.Webhook{
			generateDebugWebhook(
				config.MutatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				"*/*",
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) constructMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name:   config.MutatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			generateWebhook(
				config.MutatingWebhookName,
				config.MutatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				"*/*",
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create},
			),
		},
	}
}
