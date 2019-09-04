package webhookconfig

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) contructDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.MutatingWebhookServicePath)
	glog.V(3).Infof("Debug MutatingWebhookConfig is registered with url %s\n", url)

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationDebugName,
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
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) constructMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationName,
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
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

//RemoveResourceMutatingWebhookConfiguration removes mutating webhook configuration for all resources
func (wrc *WebhookRegistrationClient) RemoveResourceMutatingWebhookConfiguration() error {
	var configName string
	if wrc.serverIP != "" {
		configName = config.MutatingWebhookConfigurationDebugName
	} else {
		configName = config.MutatingWebhookConfigurationName
	}
	// delete webhook configuration
	err := wrc.registrationClient.MutatingWebhookConfigurations().Delete(configName, &v1.DeleteOptions{})
	if errors.IsNotFound(err) {
		glog.V(4).Infof("resource webhook configuration %s does not exits, so not deleting", configName)
		return nil
	}
	if err != nil {
		glog.V(4).Infof("failed to delete resource webhook configuration %s: %v", configName, err)
		return err
	}
	glog.V(4).Infof("deleted resource webhook configuration %s", configName)
	return nil
}
