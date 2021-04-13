package webhookconfig

import (
	"fmt"
	"sync"

	"github.com/kyverno/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *Register) constructDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
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
				[]string{"*/*"},
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *Register) constructMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {

	webhookCfg := generateMutatingWebhook(
		config.MutatingWebhookName,
		config.MutatingWebhookServicePath,
		caData, false, wrc.timeoutSeconds,
		[]string{"*/*"}, "*", "*",
		[]admregapi.OperationType{admregapi.Create, admregapi.Update})

	reinvoke := admregapi.IfNeededReinvocationPolicy
	webhookCfg.ReinvocationPolicy = &reinvoke

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.MutatingWebhook{webhookCfg},
	}
}

//getResourceMutatingWebhookConfigName returns the webhook configuration name
func (wrc *Register) getResourceMutatingWebhookConfigName() string {
	if wrc.serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
}

func (wrc *Register) removeResourceMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	configName := wrc.getResourceMutatingWebhookConfigName()
	logger := wrc.log.WithValues("kind", kindMutating, "name", configName)

	if mutateCache, ok := wrc.resCache.GetGVRCache("MutatingWebhookConfiguration"); ok {
		if _, err := mutateCache.Lister().Get(configName); err != nil && errorsapi.IsNotFound(err) {
			logger.V(4).Info("webhook not found")
			return
		}
	}

	// delete webhook configuration
	err := wrc.client.DeleteResource("", kindMutating, "", configName, false)
	if errors.IsNotFound(err) {
		logger.V(4).Info("webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete the mutating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}

func (wrc *Register) constructDebugValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
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
				[]string{"*/*"},
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete},
			),
		},
	}
}

func (wrc *Register) constructValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
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
				[]string{"*/*"},
				"*",
				"*",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete},
			),
		},
	}
}

// getResourceValidatingWebhookConfigName returns the webhook configuration name
func (wrc *Register) getResourceValidatingWebhookConfigName() string {
	if wrc.serverIP != "" {
		return config.ValidatingWebhookConfigurationDebugName
	}

	return config.ValidatingWebhookConfigurationName
}

func (wrc *Register) removeResourceValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	configName := wrc.getResourceValidatingWebhookConfigName()
	logger := wrc.log.WithValues("kind", kindValidating, "name", configName)

	if mutateCache, ok := wrc.resCache.GetGVRCache("ValidatingWebhookConfiguration"); ok {
		if _, err := mutateCache.Lister().Get(configName); err != nil && errorsapi.IsNotFound(err) {
			logger.V(4).Info("webhook not found")
			return
		}
	}

	err := wrc.client.DeleteResource("", kindValidating, "", configName, false)
	if errors.IsNotFound(err) {
		logger.V(5).Info("webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete the validating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
	return
}
