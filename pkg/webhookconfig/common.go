package webhookconfig

import (
	"io/ioutil"

	"github.com/nirmata/kyverno/pkg/config"
	admregapi "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

func (wrc *WebhookRegistrationClient) readCaData() []byte {
	logger := wrc.log
	var caData []byte
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData = wrc.client.ReadRootCASecret(); len(caData) != 0 {
		logger.V(4).Info("read CA from secret")
		return caData
	}
	logger.V(4).Info("failed to read CA from secret, reading from kubeconfig")
	// load the CA from kubeconfig
	if caData = extractCA(wrc.clientConfig); len(caData) != 0 {
		logger.V(4).Info("read CA from kubeconfig")
		return caData
	}
	logger.V(4).Info("failed to read CA from kubeconfig")
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
	logger := wrc.log
	kubePolicyDeployment, err := wrc.client.GetKubePolicyDeployment()

	if err != nil {
		logger.Error(err, "failed to construct OwnerReference")
		return v1.OwnerReference{}
	}

	return v1.OwnerReference{
		APIVersion: config.DeploymentAPIVersion,
		Kind:       config.DeploymentKind,
		Name:       kubePolicyDeployment.ObjectMeta.Name,
		UID:        kubePolicyDeployment.ObjectMeta.UID,
	}
}

// debug mutating webhook
func generateDebugMutatingWebhook(name, url string, caData []byte, validate bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.MutatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	failurePolicy := admregapi.Ignore
	reinvocationPolicy := admregapi.NeverReinvocationPolicy

	return admregapi.MutatingWebhook{
		ReinvocationPolicy: &reinvocationPolicy,
		Name:               name,
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
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}
}

func generateDebugValidatingWebhook(name, url string, caData []byte, validate bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.ValidatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	failurePolicy := admregapi.Ignore
	return admregapi.ValidatingWebhook{
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
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}
}

// func generateWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.Webhook {
// 	sideEffect := admregapi.SideEffectClassNoneOnDryRun
// 	failurePolicy := admregapi.Ignore
// 	return admregapi.Webhook{
// 		Name: name,
// 		ClientConfig: admregapi.WebhookClientConfig{
// 			Service: &admregapi.ServiceReference{
// 				Namespace: config.KubePolicyNamespace,
// 				Name:      config.WebhookServiceName,
// 				Path:      &servicePath,
// 			},
// 			CABundle: caData,
// 		},
// 		SideEffects: &sideEffect,
// 		Rules: []admregapi.RuleWithOperations{
// 			admregapi.RuleWithOperations{
// 				Operations: operationTypes,
// 				Rule: admregapi.Rule{
// 					APIGroups: []string{
// 						apiGroups,
// 					},
// 					APIVersions: []string{
// 						apiVersions,
// 					},
// 					Resources: []string{
// 						resource,
// 					},
// 				},
// 			},
// 		},
// 		AdmissionReviewVersions: []string{"v1beta1"},
// 		TimeoutSeconds:          &timeoutSeconds,
// 		FailurePolicy:           &failurePolicy,
// 	}
// }

// mutating webhook
func generateMutatingWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.MutatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	failurePolicy := admregapi.Ignore
	reinvocationPolicy := admregapi.NeverReinvocationPolicy

	return admregapi.MutatingWebhook{
		ReinvocationPolicy: &reinvocationPolicy,
		Name:               name,
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
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}
}

// validating webhook
func generateValidatingWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, resource, apiGroups, apiVersions string, operationTypes []admregapi.OperationType) admregapi.ValidatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	failurePolicy := admregapi.Ignore
	return admregapi.ValidatingWebhook{
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
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}
}
