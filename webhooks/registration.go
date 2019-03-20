package webhooks
import (
	"io/ioutil"

	rest "k8s.io/client-go/rest"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	adm "k8s.io/api/admissionregistration/v1beta1"
	types "k8s.io/api/admissionregistration/v1beta1"
	admreg "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
)

const (
	webhookName = "nirmata-kube-policy-webhook-cfg"
	mutationWebhookName = "webhook.nirmata.kube-policy"
	webhookServiceNamespace = "default"
	webhookServiceName = "kube-policy-svc"
)

var (
	webhookPath = "mutate"
	webhookLabels = map[string]string {
	    "app": "kube-policy",
    }
)

func RegisterMutationWebhook(config *rest.Config) (*types.MutatingWebhookConfiguration, error) {
var result *types.MutatingWebhookConfiguration = nil

	registrationClient, err := admreg.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	result, err = registrationClient.MutatingWebhookConfigurations().Create(constructWebhookConfig(config))
	if err != nil {
		return nil, err
	}

	return result, nil
}

func constructWebhookConfig(config *rest.Config) *adm.MutatingWebhookConfiguration {
	return &adm.MutatingWebhookConfiguration {
		ObjectMeta: meta.ObjectMeta {
			Name: webhookName,
			Labels: webhookLabels,
		},
		Webhooks: []adm.Webhook {
			adm.Webhook {
				Name: mutationWebhookName,
				ClientConfig: adm.WebhookClientConfig {
                    Service: &adm.ServiceReference {
						Namespace: webhookServiceNamespace,
						Name: webhookServiceName,
						Path: &webhookPath,
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