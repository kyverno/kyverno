package webhookconfig

import (
	"fmt"
	"sync"

	"github.com/kyverno/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *Register) defaultResourceWebhookRule() admregapi.Rule {
	if wrc.autoUpdateWebhooks {
		return admregapi.Rule{}
	}

	return admregapi.Rule{
		Resources:   []string{"*/*"},
		APIGroups:   []string{"*"},
		APIVersions: []string{"*"},
	}
}

func (wrc *Register) constructDefaultDebugMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	logger := wrc.log
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.MutatingWebhookServicePath)
	logger.V(4).Info("Debug MutatingWebhookConfig registered", "url", url)
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(
				config.MutatingWebhookName+"-ignore",
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
			generateDebugMutatingWebhook(
				config.MutatingWebhookName+"-fail",
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Fail,
			),
		},
	}
}

func (wrc *Register) constructDefaultMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.MutatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(
				config.MutatingWebhookName+"-ignore",
				config.MutatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Ignore,
			),
			generateMutatingWebhook(
				config.MutatingWebhookName+"-fail",
				config.MutatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
				admregapi.Fail,
			),
		},
	}
}

//getResourceMutatingWebhookConfigName returns the webhook configuration name
func getResourceMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
}

func (wrc *Register) removeResourceMutatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	configName := getResourceMutatingWebhookConfigName(wrc.serverIP)
	logger := wrc.log.WithValues("kind", kindMutating, "name", configName)

	if _, err := wrc.mwcLister.Get(configName); err != nil && errorsapi.IsNotFound(err) {
		logger.V(4).Info("webhook not found")
		return
	}

	// delete webhook configuration
	err := wrc.client.DeleteResource("", kindMutating, "", configName, false)
	if errorsapi.IsNotFound(err) {
		logger.V(4).Info("webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete the mutating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}

func (wrc *Register) constructDefaultDebugValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	url := fmt.Sprintf("https://%s%s", wrc.serverIP, config.ValidatingWebhookServicePath)

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.ValidatingWebhookConfigurationDebugName,
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateDebugValidatingWebhook(
				config.ValidatingWebhookName+"-ignore",
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete, admregapi.Connect},
				admregapi.Ignore,
			),
			generateDebugValidatingWebhook(
				config.ValidatingWebhookName+"-fail",
				url,
				caData,
				true,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete, admregapi.Connect},
				admregapi.Fail,
			),
		},
	}
}

func (wrc *Register) constructDefaultValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {
	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.ValidatingWebhookConfigurationName,
			OwnerReferences: []v1.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(
				config.ValidatingWebhookName+"-ignore",
				config.ValidatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete, admregapi.Connect},
				admregapi.Ignore,
			),
			generateValidatingWebhook(
				config.ValidatingWebhookName+"-fail",
				config.ValidatingWebhookServicePath,
				caData,
				false,
				wrc.timeoutSeconds,
				wrc.defaultResourceWebhookRule(),
				[]admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete, admregapi.Connect},
				admregapi.Fail,
			),
		},
	}
}

// getResourceValidatingWebhookConfigName returns the webhook configuration name
func getResourceValidatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.ValidatingWebhookConfigurationDebugName
	}

	return config.ValidatingWebhookConfigurationName
}

func (wrc *Register) removeResourceValidatingWebhookConfiguration(wg *sync.WaitGroup) {
	defer wg.Done()

	configName := getResourceValidatingWebhookConfigName(wrc.serverIP)
	logger := wrc.log.WithValues("kind", kindValidating, "name", configName)

	if _, err := wrc.vwcLister.Get(configName); err != nil && errorsapi.IsNotFound(err) {
		logger.V(4).Info("webhook not found")
		return
	}

	err := wrc.client.DeleteResource("", kindValidating, "", configName, false)
	if errorsapi.IsNotFound(err) {
		logger.V(5).Info("webhook configuration not found")
		return
	}

	if err != nil {
		logger.Error(err, "failed to delete the validating webhook configuration")
		return
	}

	logger.Info("webhook configuration deleted")
}
