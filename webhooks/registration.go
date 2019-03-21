package webhooks

import (
	"io/ioutil"

	"github.com/nirmata/kube-policy/config"

	rest "k8s.io/client-go/rest"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	adm "k8s.io/api/admissionregistration/v1beta1"
	admreg "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
)

func RegisterMutationWebhook(config *rest.Config) error {
	registrationClient, err := admreg.NewForConfig(config)
	if err != nil {
		return err
	}

	_, err = registrationClient.MutatingWebhookConfigurations().Create(constructWebhookConfig(config))
	if err != nil {
		return err
	}

	return nil
}

func constructWebhookConfig(configuration *rest.Config) *adm.MutatingWebhookConfiguration {
	return &adm.MutatingWebhookConfiguration {
		ObjectMeta: meta.ObjectMeta {
			Name: config.WebhookConfigName,
			Labels: config.WebhookConfigLabels,
		},
		Webhooks: []adm.Webhook {
			adm.Webhook {
				Name: config.MutationWebhookName,
				ClientConfig: adm.WebhookClientConfig {
					Service: &adm.ServiceReference {
						Namespace: config.WebhookServiceNamespace,
						Name: config.WebhookServiceName,
						Path: &config.WebhookServicePath,
					},
					CABundle: ExtractCA(configuration),
				},
				Rules: []adm.RuleWithOperations {
					adm.RuleWithOperations {
						Operations: []adm.OperationType {
							adm.Create,
						},
						Rule: adm.Rule {
							APIGroups: []string {
								"*",
							},
							APIVersions: []string {
								"*",
							},
							Resources: []string {
								"*/*",
							},
						},
					},
				},
			},
		},
	}
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