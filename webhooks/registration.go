package webhooks

import (
	"io/ioutil"

	"github.com/nirmata/kube-policy/constants"

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

func constructWebhookConfig(config *rest.Config) *adm.MutatingWebhookConfiguration {
	return &adm.MutatingWebhookConfiguration {
		ObjectMeta: meta.ObjectMeta {
			Name: constants.WebhookConfigName,
			Labels: constants.WebhookConfigLabels,
		},
		Webhooks: []adm.Webhook {
			adm.Webhook {
				Name: constants.MutationWebhookName,
				ClientConfig: adm.WebhookClientConfig {
                    Service: &adm.ServiceReference {
						Namespace: constants.WebhookServiceNamespace,
						Name: constants.WebhookServiceName,
						Path: &constants.WebhookServicePath,
					},
					CABundle: ExtractCA(config),
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