package webhookconfig

import (
	"io/ioutil"
	"path/filepath"
	"reflect"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	admregapi "k8s.io/api/admissionregistration/v1"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	rest "k8s.io/client-go/rest"
)

func (wrc *Register) readCaData() []byte {
	logger := wrc.log.WithName("readCaData")
	var caData []byte
	var err error

	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData, err = tls.ReadRootCASecret(wrc.clientConfig, wrc.client); err == nil {
		logger.V(4).Info("read CA from secret")
		return caData
	}

	logger.V(4).Info("failed to read CA from secret, reading from kubeconfig", "reason", err.Error())
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
		fileName = filepath.Clean(fileName)
		// We accept the risk of including a user provided file here.
		result, err := ioutil.ReadFile(fileName) // #nosec G304

		if err != nil {
			return nil
		}

		return result
	}

	return config.TLSClientConfig.CAData
}

func (wrc *Register) constructOwner() v1.OwnerReference {
	logger := wrc.log

	kubeClusterRole, err := wrc.GetKubePolicyClusterRole()
	if err != nil {
		logger.Error(err, "failed to construct OwnerReference")
		return v1.OwnerReference{}
	}

	return v1.OwnerReference{
		APIVersion: config.ClusterRoleAPIVersion,
		Kind:       config.ClusterRoleKind,
		Name:       config.ClusterRoleName,
		UID:        kubeClusterRole.GetUID(),
	}
}

func (wrc *Register) GetKubePolicyClusterRole() (*unstructured.Unstructured, error) {
	kubeNamespace, err := wrc.client.GetResource(config.ClusterRoleAPIVersion, config.ClusterRoleKind, "", config.ClusterRoleName)
	if err != nil {
		return nil, err
	}

	return kubeNamespace, nil
}

// GetKubePolicyDeployment gets Kyverno deployment using the resource cache
// it does not initialize any client call
func (wrc *Register) GetKubePolicyDeployment() (*apps.Deployment, *unstructured.Unstructured, error) {
	lister, _ := wrc.resCache.GetGVRCache("Deployment")
	kubePolicyDeployment, err := lister.NamespacedLister(config.KyvernoNamespace).Get(config.KyvernoDeploymentName)
	if err != nil {
		return nil, nil, err
	}
	deploy := apps.Deployment{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(kubePolicyDeployment.UnstructuredContent(), &deploy); err != nil {
		return nil, kubePolicyDeployment, err
	}
	return &deploy, kubePolicyDeployment, nil
}

// debug mutating webhook
func generateDebugMutatingWebhook(name, url string, caData []byte, validate bool, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.MutatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	reinvocationPolicy := admregapi.NeverReinvocationPolicy

	w := admregapi.MutatingWebhook{
		ReinvocationPolicy: &reinvocationPolicy,
		Name:               name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		SideEffects:             &sideEffect,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}

	if !reflect.DeepEqual(rule, admregapi.Rule{}) {
		w.Rules = []admregapi.RuleWithOperations{
			{
				Operations: operationTypes,
				Rule:       rule,
			},
		}
	}

	return w
}

func generateDebugValidatingWebhook(name, url string, caData []byte, validate bool, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.ValidatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	w := admregapi.ValidatingWebhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		SideEffects:             &sideEffect,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}

	if !reflect.DeepEqual(rule, admregapi.Rule{}) {
		w.Rules = []admregapi.RuleWithOperations{
			{
				Operations: operationTypes,
				Rule:       rule,
			},
		}
	}
	return w
}

// mutating webhook
func generateMutatingWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.MutatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	reinvocationPolicy := admregapi.IfNeededReinvocationPolicy

	w := admregapi.MutatingWebhook{
		ReinvocationPolicy: &reinvocationPolicy,
		Name:               name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KyvernoNamespace,
				Name:      config.KyvernoServiceName,
				Path:      &servicePath,
			},
			CABundle: caData,
		},
		SideEffects:             &sideEffect,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}

	if !reflect.DeepEqual(rule, admregapi.Rule{}) {
		w.Rules = []admregapi.RuleWithOperations{
			{
				Operations: operationTypes,
				Rule:       rule,
			},
		}
	}
	return w
}

// validating webhook
func generateValidatingWebhook(name, servicePath string, caData []byte, validation bool, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.ValidatingWebhook {
	sideEffect := admregapi.SideEffectClassNoneOnDryRun
	w := admregapi.ValidatingWebhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KyvernoNamespace,
				Name:      config.KyvernoServiceName,
				Path:      &servicePath,
			},
			CABundle: caData,
		},
		SideEffects:             &sideEffect,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
	}

	if !reflect.DeepEqual(rule, admregapi.Rule{}) {
		w.Rules = []admregapi.RuleWithOperations{
			{
				Operations: operationTypes,
				Rule:       rule,
			},
		}
	}
	return w
}
