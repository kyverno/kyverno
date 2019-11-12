package webhookconfig

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) constructVerifyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.VerifyMutatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			generateWebhook(
				config.VerifyMutatingWebhookName,
				config.VerifyMutatingWebhookServicePath,
				caData,
				true,
				wrc.timeoutSeconds,
				"deployments/*",
				"apps",
				"v1",
				[]admregapi.OperationType{admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) constructDebugVerifyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.VerifyMutatingWebhookServicePath)
	glog.V(4).Infof("Debug VerifyMutatingWebhookConfig is registered with url %s\n", url)
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.VerifyMutatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.Webhook{
			generateDebugWebhook(
				config.VerifyMutatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				"deployments/*",
				"apps",
				"v1",
				[]admregapi.OperationType{admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) removeVerifyWebhookMutatingWebhookConfig() {
	// Muating webhook configuration
	var err error
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.VerifyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.VerifyMutatingWebhookConfigurationName
	}
	glog.V(4).Infof("removing webhook configuration %s", mutatingConfig)
	err = wrc.registrationClient.MutatingWebhookConfigurations().Delete(mutatingConfig, &v1.DeleteOptions{})
	if errorsapi.IsNotFound(err) {
		glog.V(4).Infof("verify webhook configuration %s, does not exits. not deleting", mutatingConfig)
	} else if err != nil {
		glog.Errorf("failed to delete verify webhook configuration %s: %v", mutatingConfig, err)
	} else {
		glog.V(4).Infof("succesfully deleted verify webhook configuration %s", mutatingConfig)
	}
}
