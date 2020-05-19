package webhookconfig

import (
	"fmt"

	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) constructDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	logger := wrc.log
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.MutatingWebhookServicePath)
	logger.V(4).Info("Debug MutatingWebhookConfig registered", "url", url)
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(
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
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(
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

//GetResourceMutatingWebhookConfigName returns the webhook configuration name
func (wrc *WebhookRegistrationClient) GetResourceMutatingWebhookConfigName() string {
	if wrc.serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
}

//RemoveResourceMutatingWebhookConfiguration removes mutating webhook configuration for all resources
func (wrc *WebhookRegistrationClient) RemoveResourceMutatingWebhookConfiguration() error {
	configName := wrc.GetResourceMutatingWebhookConfigName()
	logger := wrc.log.WithValues("kind", MutatingWebhookConfigurationKind, "name", configName)
	// delete webhook configuration
	err := wrc.client.DeleteResource(MutatingWebhookConfigurationKind, "", configName, false)
	if errors.IsNotFound(err) {
		logger.V(5).Info("webhook configuration not found")
		return nil
	}

	if err != nil {
		logger.V(4).Info("failed to delete webhook configuration")
		return err
	}

	logger.V(4).Info("deleted webhook configuration")
	return nil
}

func (wrc *WebhookRegistrationClient) constructDebugValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.ValidatingWebhookServicePath)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.ValidatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateDebugValidatingWebhook(
				config.ValidatingWebhookName,
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				"*/*",
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) constructValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.ValidatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(
				config.ValidatingWebhookName,
				config.ValidatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				"*/*",
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete},
			),
		},
	}
}

// GetResourceValidatingWebhookConfigName returns the webhook configuration name
func (wrc *WebhookRegistrationClient) GetResourceValidatingWebhookConfigName() string {
	if wrc.serverIP != "" {
		return config.ValidatingWebhookConfigurationDebugName
	}

	return config.ValidatingWebhookConfigurationName
}

// RemoveResourceValidatingWebhookConfiguration deletes an existing webhook configuration
func (wrc *WebhookRegistrationClient) RemoveResourceValidatingWebhookConfiguration() error {
	configName := wrc.GetResourceValidatingWebhookConfigName()
	logger := wrc.log.WithValues("kind", ValidatingWebhookConfigurationKind, "name", configName)
	err := wrc.client.DeleteResource(ValidatingWebhookConfigurationKind, "", configName, false)
	if errors.IsNotFound(err) {
		logger.V(5).Info("webhook configuration not found")
		return nil
	}

	if err != nil {
		logger.Error(err, "failed to delete the webhook configuration")
		return err
	}

	logger.Info("webhook configuration deleted")
	return nil
}
