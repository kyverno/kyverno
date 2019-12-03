package webhookconfig

import (
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

func (wrc *WebhookRegistrationClient) readCaData() []byte {
	var caData []byte
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData = wrc.client.ReadRootCASecret(); len(caData) != 0 {
		glog.V(4).Infof("read CA from secret")
		return caData
	}
	glog.V(4).Infof("failed to read CA from secret, reading from kubeconfig")
	// load the CA from kubeconfig
	if caData = extractCA(wrc.clientConfig); len(caData) != 0 {
		glog.V(4).Infof("read CA from kubeconfig")
		return caData
	}
	glog.V(4).Infof("failed to read CA from kubeconfig")
	return nil
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

func (wrc *WebhookRegistrationClient) constructOwner() v1.OwnerReference {
	kubePolicyDeployment, err := wrc.client.GetKubePolicyDeployment()

	if err != nil {
		glog.Errorf("Error when constructing OwnerReference, err: %v\n", err)
		return v1.OwnerReference{}
	}

	return v1.OwnerReference{
		APIVersion: config.DeploymentAPIVersion,
		Kind:       config.DeploymentKind,
		Name:       kubePolicyDeployment.ObjectMeta.Name,
		UID:        kubePolicyDeployment.ObjectMeta.UID,
	}
}

func generateDebugWebhook(name, url string, caData []byte, validate bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.Webhook {
	sideEffect := admregapi.SideEffectClassSome
	return admregapi.Webhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		SideEffects: &sideEffect,
		Rules: []admregapi.RuleWithOperations{
			admregapi.RuleWithOperations{
				Operations: operationTypes,

				Rule: admregapi.Rule{
					APIGroups: []string{
						apiGroups,
					},
					APIVersions: []string{
						apiVersions,
					},
					Resources: []string{
						resource,
					},
				},
			},
		},
		TimeoutSeconds: &timeoutSeconds,
	}
}

func generateWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.Webhook {
	sideEffect := admregapi.SideEffectClassSome
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
		SideEffects: &sideEffect,
		Rules: []admregapi.RuleWithOperations{
			admregapi.RuleWithOperations{
				Operations: operationTypes,
				Rule: admregapi.Rule{
					APIGroups: []string{
						apiGroups,
					},
					APIVersions: []string{
						apiVersions,
					},
					Resources: []string{
						resource,
					},
				},
			},
		},
		TimeoutSeconds: &timeoutSeconds,
	}
}
