package webhookconfig

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	errorsapi "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (wrc *WebhookRegistrationClient) contructPolicyValidatingWebhookConfig(caData []byte) *admregapi.ValidatingWebhookConfiguration {

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyValidatingWebhookConfigurationName,
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
				"v1",
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
			Name: config.PolicyValidatingWebhookConfigurationDebugName,
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
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

func (wrc *WebhookRegistrationClient) contructPolicyMutatingWebhookConfig(caData []byte) *admregapi.MutatingWebhookConfiguration {
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: v1.ObjectMeta{
			Name: config.PolicyMutatingWebhookConfigurationName,
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
				"v1",
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
			Name: config.PolicyMutatingWebhookConfigurationDebugName,
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
				"v1",
				[]admregapi.OperationType{admregapi.Create, admregapi.Update},
			),
		},
	}
}

// removePolicyWebhookConfigurations removes mutating and validating webhook configurations, if already presnt
// webhookConfigurations are re-created later
func (wrc *WebhookRegistrationClient) removePolicyWebhookConfigurations() {
	// Validating webhook configuration
	var err error
	var validatingConfig string
	if wrc.serverIP != "" {
		validatingConfig = config.PolicyValidatingWebhookConfigurationDebugName
	} else {
		validatingConfig = config.PolicyValidatingWebhookConfigurationName
	}
	glog.V(4).Infof("removing webhook configuration %s", validatingConfig)
	err = wrc.registrationClient.ValidatingWebhookConfigurations().Delete(validatingConfig, &v1.DeleteOptions{})
	if errorsapi.IsNotFound(err) {
		glog.V(4).Infof("policy webhook configuration %s, does not exits. not deleting", validatingConfig)
	} else if err != nil {
		glog.Errorf("failed to delete policy webhook configuration %s: %v", validatingConfig, err)
	} else {
		glog.V(4).Infof("succesfully deleted policy webhook configuration %s", validatingConfig)
	}

	// Mutating webhook configuration
	var mutatingConfig string
	if wrc.serverIP != "" {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationDebugName
	} else {
		mutatingConfig = config.PolicyMutatingWebhookConfigurationName
	}

	glog.V(4).Infof("removing webhook configuration %s", mutatingConfig)
	err = wrc.registrationClient.MutatingWebhookConfigurations().Delete(mutatingConfig, &v1.DeleteOptions{})
	if errorsapi.IsNotFound(err) {
		glog.V(4).Infof("policy webhook configuration %s, does not exits. not deleting", mutatingConfig)
	} else if err != nil {
		glog.Errorf("failed to delete policy webhook configuration %s: %v", mutatingConfig, err)
	} else {
		glog.V(4).Infof("succesfully deleted policy webhook configuration %s", mutatingConfig)
	}
}
