package webhooks

import (
	"errors"
	"io/ioutil"

	kubeclient "github.com/nirmata/kube-policy/kubeclient"
	"github.com/nirmata/kube-policy/pkg/config"

	admregapi "k8s.io/api/admissionregistration/v1beta1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	admregclient "k8s.io/client-go/kubernetes/typed/admissionregistration/v1beta1"
	rest "k8s.io/client-go/rest"
)

// WebhookRegistrationClient is client for registration webhooks on cluster
type WebhookRegistrationClient struct {
	registrationClient *admregclient.AdmissionregistrationV1beta1Client
	kubeclient         *kubeclient.KubeClient
	clientConfig       *rest.Config
}

// NewWebhookRegistrationClient creates new WebhookRegistrationClient instance
func NewWebhookRegistrationClient(clientConfig *rest.Config, kubeclient *kubeclient.KubeClient) (*WebhookRegistrationClient, error) {
	registrationClient, err := admregclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return &WebhookRegistrationClient{
		registrationClient: registrationClient,
		kubeclient:         kubeclient,
		clientConfig:       clientConfig,
	}, nil
}

// Register creates admission webhooks configs on cluster
func (wrc *WebhookRegistrationClient) Register() error {
	// For the case if cluster already has this configs
	wrc.Deregister()

	mutatingWebhookConfig, err := wrc.constructMutatingWebhookConfig(wrc.clientConfig)
	if err != nil {
		return err
	}

	_, err = wrc.registrationClient.MutatingWebhookConfigurations().Create(mutatingWebhookConfig)
	if err != nil {
		return err
	}

	validationWebhookConfig, err := wrc.constructValidatingWebhookConfig(wrc.clientConfig)
	if err != nil {
		return err
	}

	_, err = wrc.registrationClient.ValidatingWebhookConfigurations().Create(validationWebhookConfig)
	if err != nil {
		return err
	}

	return nil
}

// Deregister deletes webhook configs from cluster
// This function does not fail on error:
// Register will fail if the config exists, so there is no need to fail on error
func (wrc *WebhookRegistrationClient) Deregister() {
	wrc.registrationClient.MutatingWebhookConfigurations().Delete(config.MutatingWebhookConfigurationName, &meta.DeleteOptions{})
	wrc.registrationClient.ValidatingWebhookConfigurations().Delete(config.ValidatingWebhookConfigurationName, &meta.DeleteOptions{})
}

func (wrc *WebhookRegistrationClient) constructMutatingWebhookConfig(configuration *rest.Config) (*admregapi.MutatingWebhookConfiguration, error) {
	caData := extractCA(configuration)
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: meta.ObjectMeta{
			Name:   config.MutatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []meta.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			constructWebhook(
				config.MutatingWebhookName,
				config.MutatingWebhookServicePath,
				caData),
		},
	}, nil
}

func (wrc *WebhookRegistrationClient) constructValidatingWebhookConfig(configuration *rest.Config) (*admregapi.ValidatingWebhookConfiguration, error) {
	caData := extractCA(configuration)
	if len(caData) == 0 {
		return nil, errors.New("Unable to extract CA data from configuration")
	}

	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: meta.ObjectMeta{
			Name:   config.ValidatingWebhookConfigurationName,
			Labels: config.KubePolicyAppLabels,
			OwnerReferences: []meta.OwnerReference{
				wrc.constructOwner(),
			},
		},
		Webhooks: []admregapi.Webhook{
			constructWebhook(
				config.ValidatingWebhookName,
				config.ValidatingWebhookServicePath,
				caData),
		},
	}, nil
}

func constructWebhook(name, servicePath string, caData []byte) admregapi.Webhook {
	return admregapi.Webhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KubePolicyNamespace,
				Name:      config.WebhookServiceName,
				Path:      &servicePath,
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
	}
}

func (wrc *WebhookRegistrationClient) constructOwner() meta.OwnerReference {
	kubePolicyDeployment, err := wrc.kubeclient.GetKubePolicyDeployment()

	if err != nil {
		return meta.OwnerReference{}
	}

	return meta.OwnerReference{
		APIVersion: config.DeploymentAPIVersion,
		Kind:       config.DeploymentKind,
		Name:       kubePolicyDeployment.ObjectMeta.Name,
		UID:        kubePolicyDeployment.ObjectMeta.UID,
	}
}

// ExtractCA used for extraction CA from config
func extractCA(config *rest.Config) (result []byte) {
	fileName := config.TLSClientConfig.CAFile

	if fileName != "" {
		result, err := ioutil.ReadFile(fileName)

		if err != nil {
			return nil
		}

		return result
	}

	return config.TLSClientConfig.CAData
}
