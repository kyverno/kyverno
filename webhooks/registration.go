package webhooks

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/nirmata/kube-policy/config"

	admregapi "k8s.io/api/admissionregistration/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	admregclient "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	rest "k8s.io/client-go/rest"
)

type MutationWebhookRegistration struct {
	registrationClient *admregclient.AdmissionregistrationV1beta1Client
}

func NewMutationWebhookRegistration(clientConfig *rest.Config) (*MutationWebhookRegistration, error) {
	registrationClient, err := admregclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	webhookConfig, err := constructWebhookConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	oldConfig, err := registrationClient.MutatingWebhookConfigurations().Get(config.WebhookConfigName, meta.GetOptions{})
	if oldConfig != nil && oldConfig.ObjectMeta.UID != "" {
		// Normally webhook configuration should be deleted from cluster when controller end his work.
		// But if old configuration is detected in cluster, it should be replaced by new one.
		err = registrationClient.MutatingWebhookConfigurations().Delete(config.WebhookConfigName, &meta.DeleteOptions{})
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Failed to delete old webhook configuration: %v", err))
		}
	}

	_, err = registrationClient.MutatingWebhookConfigurations().Create(webhookConfig)
	if err != nil {
		return nil, err
	}

	return &MutationWebhookRegistration{
		registrationClient: registrationClient,
	}, nil
}

func (mwr *MutationWebhookRegistration) Deregister() error {
	return mwr.registrationClient.MutatingWebhookConfigurations().Delete(config.MutationWebhookName, &meta.DeleteOptions{})
}

func constructWebhookConfig(configuration *rest.Config) (*admregapi.MutatingWebhookConfiguration, error) {
	caData := ExtractCA(configuration)
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: meta.ObjectMeta{
			Name:   config.WebhookConfigName,
			Labels: config.WebhookConfigLabels,
		},
		Webhooks: []admregapi.Webhook{
			admregapi.Webhook{
				Name: config.MutationWebhookName,
				ClientConfig: admregapi.WebhookClientConfig{
					Service: &admregapi.ServiceReference{
						Namespace: config.WebhookServiceNamespace,
						Name:      config.WebhookServiceName,
						Path:      &config.WebhookServicePath,
					},
					CABundle: caData,
				},
				Rules: []admregapi.RuleWithOperations{
					admregapi.RuleWithOperations{
						Operations: []admregapi.OperationType{
							admregapi.Create,
						},
						Rule: admregapi.Rule{
							APIGroups: []string{
								"*",
							},
							APIVersions: []string{
								"*",
							},
							Resources: []string{
								"*/*",
							},
						},
					},
				},
			},
		},
	}, nil
}

func ExtractCA(config *rest.Config) (result []byte) {
	fileName := config.TLSClientConfig.CAFile

	if fileName != "" {
		result, err := ioutil.ReadFile(fileName)

		if err != nil {
			return nil
		}

		return result
	} else {
		return config.TLSClientConfig.CAData
	}
}
