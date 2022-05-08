package webhookconfig

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	admregapi "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var (
	noneOnDryRun = admregapi.SideEffectClassNoneOnDryRun
	never        = admregapi.NeverReinvocationPolicy
	ifNeeded     = admregapi.IfNeededReinvocationPolicy
	policyRule   = admregapi.Rule{
		Resources:   []string{"clusterpolicies/*", "policies/*"},
		APIGroups:   []string{"kyverno.io"},
		APIVersions: []string{"v1"},
	}
	verifyRule = admregapi.Rule{
		Resources:   []string{"leases"},
		APIGroups:   []string{"coordination.k8s.io"},
		APIVersions: []string{"v1"},
	}
	vertifyObjectSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "kyverno",
		},
	}
	update       = []admregapi.OperationType{admregapi.Update}
	createUpdate = []admregapi.OperationType{admregapi.Create, admregapi.Update}
	all          = []admregapi.OperationType{admregapi.Create, admregapi.Update, admregapi.Delete, admregapi.Connect}
)

func (wrc *Register) readCaData() []byte {
	logger := wrc.log.WithName("readCaData")
	var caData []byte
	var err error

	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData, err = tls.ReadRootCASecret(wrc.clientConfig, wrc.kubeClient); err == nil {
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

func (wrc *Register) constructOwner() metav1.OwnerReference {
	logger := wrc.log
	kubeClusterRoleName, err := wrc.GetKubePolicyClusterRoleName()
	if err != nil {
		logger.Error(err, "failed to get cluster role")
		return metav1.OwnerReference{}
	}
	return metav1.OwnerReference{
		APIVersion: config.ClusterRoleAPIVersion,
		Kind:       config.ClusterRoleKind,
		Name:       kubeClusterRoleName.GetName(),
		UID:        kubeClusterRoleName.GetUID(),
	}
}

func (wrc *Register) GetKubePolicyClusterRoleName() (*corev1.ClusterRole, error) {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": "kyverno",
		},
	}
	clusterRoles, err := wrc.kubeClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)})
	if err != nil {
		return nil, err
	}
	for _, cr := range clusterRoles.Items {
		if strings.HasSuffix(cr.GetName(), "webhook") {
			return &cr, nil
		}
	}
	return nil, errors.New("failed to get cluster role with suffix webhook")
}

// GetKubePolicyDeployment gets Kyverno deployment using the resource cache
// it does not initialize any client call
func (wrc *Register) GetKubePolicyDeployment() (*appsv1.Deployment, error) {
	deploy, err := wrc.kDeplLister.Deployments(config.KyvernoNamespace).Get(config.KyvernoDeploymentName)
	if err != nil {
		return nil, err
	}
	return deploy, nil
}

// webhook utils

func generateRules(rule admregapi.Rule, operationTypes []admregapi.OperationType) []admregapi.RuleWithOperations {
	if !reflect.DeepEqual(rule, admregapi.Rule{}) {
		return []admregapi.RuleWithOperations{{Operations: operationTypes, Rule: rule}}
	}
	return nil
}

func generateDebugMutatingWebhook(name, url string, caData []byte, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.MutatingWebhook {
	return admregapi.MutatingWebhook{
		ReinvocationPolicy: &never,
		Name:               name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
		Rules:                   generateRules(rule, operationTypes),
	}
}

func generateDebugValidatingWebhook(name, url string, caData []byte, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.ValidatingWebhook {
	return admregapi.ValidatingWebhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			URL:      &url,
			CABundle: caData,
		},
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
		Rules:                   generateRules(rule, operationTypes),
	}
}

func generateMutatingWebhook(name, servicePath string, caData []byte, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.MutatingWebhook {
	return admregapi.MutatingWebhook{
		ReinvocationPolicy: &ifNeeded,
		Name:               name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KyvernoNamespace,
				Name:      config.KyvernoServiceName,
				Path:      &servicePath,
			},
			CABundle: caData,
		},
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
		Rules:                   generateRules(rule, operationTypes),
	}
}

func generateValidatingWebhook(name, servicePath string, caData []byte, timeoutSeconds int32, rule admregapi.Rule, operationTypes []admregapi.OperationType, failurePolicy admregapi.FailurePolicyType) admregapi.ValidatingWebhook {
	return admregapi.ValidatingWebhook{
		Name: name,
		ClientConfig: admregapi.WebhookClientConfig{
			Service: &admregapi.ServiceReference{
				Namespace: config.KyvernoNamespace,
				Name:      config.KyvernoServiceName,
				Path:      &servicePath,
			},
			CABundle: caData,
		},
		SideEffects:             &noneOnDryRun,
		AdmissionReviewVersions: []string{"v1beta1"},
		TimeoutSeconds:          &timeoutSeconds,
		FailurePolicy:           &failurePolicy,
		Rules:                   generateRules(rule, operationTypes),
	}
}

func generateObjectMeta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:            name,
		OwnerReferences: owner,
	}
}

// policy webhook configuration utils

func getPolicyMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.PolicyMutatingWebhookConfigurationDebugName
	}
	return config.PolicyMutatingWebhookConfigurationName
}

func getPolicyValidatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.PolicyValidatingWebhookConfigurationDebugName
	}
	return config.PolicyValidatingWebhookConfigurationName
}

func constructPolicyValidatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.ValidatingWebhookConfiguration {
	name, path := config.PolicyValidatingWebhookName, config.PolicyValidatingWebhookServicePath
	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyValidatingWebhookConfigurationName, owner),
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(name, path, caData, timeoutSeconds, policyRule, createUpdate, admregapi.Ignore),
		},
	}
}

func constructDebugPolicyValidatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.ValidatingWebhookConfiguration {
	name, url := config.PolicyValidatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.PolicyValidatingWebhookServicePath)
	return &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyValidatingWebhookConfigurationDebugName, owner),
		Webhooks: []admregapi.ValidatingWebhook{
			generateDebugValidatingWebhook(name, url, caData, timeoutSeconds, policyRule, createUpdate, admregapi.Ignore),
		},
	}
}

func constructPolicyMutatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, path := config.PolicyMutatingWebhookName, config.PolicyMutatingWebhookServicePath
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyMutatingWebhookConfigurationName, owner),
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(name, path, caData, timeoutSeconds, policyRule, createUpdate, admregapi.Ignore),
		},
	}
}

func constructDebugPolicyMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, url := config.PolicyMutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.PolicyMutatingWebhookServicePath)
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyMutatingWebhookConfigurationDebugName, owner),
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(name, url, caData, timeoutSeconds, policyRule, createUpdate, admregapi.Ignore),
		},
	}
}

// resource webhook configuration utils

func getResourceMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
}

func getResourceValidatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.ValidatingWebhookConfigurationDebugName
	}
	return config.ValidatingWebhookConfigurationName
}

func defaultResourceWebhookRule(autoUpdate bool) admregapi.Rule {
	if autoUpdate {
		return admregapi.Rule{}
	}
	return admregapi.Rule{
		APIGroups:   []string{"*"},
		APIVersions: []string{"*"},
		Resources:   []string{"*/*"},
	}
}

func constructDefaultDebugMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, url := config.MutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.MutatingWebhookServicePath)
	webhook := &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationDebugName, owner),
		Webhooks: []admregapi.MutatingWebhook{
			generateDebugMutatingWebhook(name+"-ignore", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admregapi.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateDebugMutatingWebhook(name+"-fail", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admregapi.Fail))
	}
	return webhook
}

func constructDefaultMutatingWebhookConfig(caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, path := config.MutatingWebhookName, config.MutatingWebhookServicePath
	webhook := &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationName, owner),
		Webhooks: []admregapi.MutatingWebhook{
			generateMutatingWebhook(name+"-ignore", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admregapi.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateMutatingWebhook(name+"-fail", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admregapi.Fail))
	}
	return webhook
}

func constructDefaultDebugValidatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admregapi.ValidatingWebhookConfiguration {
	name, url := config.ValidatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.ValidatingWebhookServicePath)
	webhook := &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.ValidatingWebhookConfigurationDebugName, owner),
		Webhooks: []admregapi.ValidatingWebhook{
			generateDebugValidatingWebhook(name+"-ignore", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admregapi.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateDebugValidatingWebhook(name+"-fail", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admregapi.Fail))
	}
	return webhook
}

func constructDefaultValidatingWebhookConfig(caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admregapi.ValidatingWebhookConfiguration {
	name, path := config.ValidatingWebhookName, config.ValidatingWebhookServicePath
	webhook := &admregapi.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.ValidatingWebhookConfigurationName, owner),
		Webhooks: []admregapi.ValidatingWebhook{
			generateValidatingWebhook(name+"-ignore", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admregapi.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateValidatingWebhook(name+"-fail", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admregapi.Fail))
	}
	return webhook
}

// verify webhook configuration utils

func getVerifyMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.VerifyMutatingWebhookConfigurationDebugName
	}
	return config.VerifyMutatingWebhookConfigurationName
}

func constructVerifyMutatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, path := config.VerifyMutatingWebhookName, config.VerifyMutatingWebhookServicePath
	webhook := generateMutatingWebhook(name, path, caData, timeoutSeconds, verifyRule, update, admregapi.Ignore)
	webhook.ObjectSelector = vertifyObjectSelector
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.VerifyMutatingWebhookConfigurationName, owner),
		Webhooks:   []admregapi.MutatingWebhook{webhook},
	}
}

func constructDebugVerifyMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admregapi.MutatingWebhookConfiguration {
	name, url := config.VerifyMutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.VerifyMutatingWebhookServicePath)
	webhook := generateDebugMutatingWebhook(name, url, caData, timeoutSeconds, verifyRule, update, admregapi.Ignore)
	webhook.ObjectSelector = vertifyObjectSelector
	return &admregapi.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.VerifyMutatingWebhookConfigurationDebugName, owner),
		Webhooks:   []admregapi.MutatingWebhook{webhook},
	}
}
