package webhooks

import (
	"errors"
	"io/ioutil"

	"github.com/nirmata/kube-policy/config"
	kubeclient "github.com/nirmata/kube-policy/kubeclient"

	admregapi "k8s.io/api/admissionregistration/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	admregclient "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	rest "k8s.io/client-go/rest"
)

type MutationWebhookRegistration struct {
	registrationClient *admregclient.AdmissionregistrationV1beta1Client
	kubeclient         *kubeclient.KubeClient
	clientConfig       *rest.Config
}

func NewMutationWebhookRegistration(clientConfig *rest.Config, kubeclient *kubeclient.KubeClient) (*MutationWebhookRegistration, error) {
	registrationClient, err := admregclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return &MutationWebhookRegistration{
		registrationClient: registrationClient,
		kubeclient:         kubeclient,
		clientConfig:       clientConfig,
	}, nil
}

func (mwr *MutationWebhookRegistration) Register() error {
	webhookConfig, err := mwr.constructWebhookConfig(mwr.clientConfig)
	if err != nil {
		return err
	}

	_, err = mwr.registrationClient.MutatingWebhookConfigurations().Create(webhookConfig)
	if err != nil {
		return err
	}

	return nil
}

func (mwr *MutationWebhookRegistration) Deregister() error {
	return mwr.registrationClient.MutatingWebhookConfigurations().Delete(config.MutationWebhookName, &meta.DeleteOptions{})
}

func (mwr *MutationWebhookRegistration) constructWebhookConfig(configuration *rest.Config) (*admregapi.MutatingWebhookConfiguration, error) {
	caData := ExtractCA(configuration)
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	kubePolicyDeployment, err := mwr.kubeclient.GetKubePolicyDeployment()

	if err != nil {
		return nil, err
	}

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: meta.ObjectMeta{
			Name:   config.WebhookConfigName,
			Labels: config.WebhookConfigLabels,
			OwnerReferences: []meta.OwnerReference{
				meta.OwnerReference{
					APIVersion: config.DeploymentAPIVersion,
					Kind:       config.DeploymentKind,
					Name:       kubePolicyDeployment.ObjectMeta.Name,
					UID:        kubePolicyDeployment.ObjectMeta.UID,
				},
			},
		},
		Webhooks: []admregapi.Webhook{
			admregapi.Webhook{
				Name: config.MutationWebhookName,
				ClientConfig: admregapi.WebhookClientConfig{
					Service: &admregapi.ServiceReference{
						Namespace: config.KubePolicyNamespace,
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
