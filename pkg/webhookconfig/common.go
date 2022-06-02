package webhookconfig

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/tls"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	managedByLabel string = "webhook.kyverno.io/managed-by"
	kyvernoValue   string = "kyverno"
)

var (
	noneOnDryRun = admissionregistrationv1.SideEffectClassNoneOnDryRun
	never        = admissionregistrationv1.NeverReinvocationPolicy
	ifNeeded     = admissionregistrationv1.IfNeededReinvocationPolicy
	policyRule   = admissionregistrationv1.Rule{
		Resources:   []string{"clusterpolicies/*", "policies/*"},
		APIGroups:   []string{"kyverno.io"},
		APIVersions: []string{"v1"},
	}
	verifyRule = admissionregistrationv1.Rule{
		Resources:   []string{"leases"},
		APIGroups:   []string{"coordination.k8s.io"},
		APIVersions: []string{"v1"},
	}
	vertifyObjectSelector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": kyvernoValue,
		},
	}
	update       = []admissionregistrationv1.OperationType{admissionregistrationv1.Update}
	createUpdate = []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
	all          = []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update, admissionregistrationv1.Delete, admissionregistrationv1.Connect}
)

func (wrc *Register) readCaData() []byte {
	logger := wrc.log.WithName("readCaData")
	var caData []byte
	var err error

	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData, err = tls.ReadRootCASecret(wrc.kubeClient); err == nil {
		logger.V(4).Info("read CA from secret")
		return caData
	}

	logger.V(4).Info("failed to read CA from kubeconfig")
	return nil
}

func getHealthyPodsIP(pods []corev1.Pod) []string {
	var ips []string
	for _, pod := range pods {
		if pod.Status.Phase == "Running" {
			ips = append(ips, pod.Status.PodIP)
		}
	}
	return ips
}

func (wrc *Register) GetKubePolicyClusterRoleName() (*rbacv1.ClusterRole, error) {
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"app.kubernetes.io/name": kyvernoValue,
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
	deploy, err := wrc.kDeplLister.Deployments(config.KyvernoNamespace()).Get(config.KyvernoDeploymentName())
	if err != nil {
		return nil, err
	}
	return deploy, nil
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

// webhook utils

func generateRules(rule admissionregistrationv1.Rule, operationTypes []admissionregistrationv1.OperationType) []admissionregistrationv1.RuleWithOperations {
	if !reflect.DeepEqual(rule, admissionregistrationv1.Rule{}) {
		return []admissionregistrationv1.RuleWithOperations{{Operations: operationTypes, Rule: rule}}
	}
	return nil
}

func generateDebugMutatingWebhook(name, url string, caData []byte, timeoutSeconds int32, rule admissionregistrationv1.Rule, operationTypes []admissionregistrationv1.OperationType, failurePolicy admissionregistrationv1.FailurePolicyType) admissionregistrationv1.MutatingWebhook {
	return admissionregistrationv1.MutatingWebhook{
		ReinvocationPolicy: &never,
		Name:               name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
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

func generateDebugValidatingWebhook(name, url string, caData []byte, timeoutSeconds int32, rule admissionregistrationv1.Rule, operationTypes []admissionregistrationv1.OperationType, failurePolicy admissionregistrationv1.FailurePolicyType) admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name: name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
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

func generateMutatingWebhook(name, servicePath string, caData []byte, timeoutSeconds int32, rule admissionregistrationv1.Rule, operationTypes []admissionregistrationv1.OperationType, failurePolicy admissionregistrationv1.FailurePolicyType) admissionregistrationv1.MutatingWebhook {
	return admissionregistrationv1.MutatingWebhook{
		ReinvocationPolicy: &ifNeeded,
		Name:               name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Namespace: config.KyvernoNamespace(),
				Name:      config.KyvernoServiceName(),
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

func generateValidatingWebhook(name, servicePath string, caData []byte, timeoutSeconds int32, rule admissionregistrationv1.Rule, operationTypes []admissionregistrationv1.OperationType, failurePolicy admissionregistrationv1.FailurePolicyType) admissionregistrationv1.ValidatingWebhook {
	return admissionregistrationv1.ValidatingWebhook{
		Name: name,
		ClientConfig: admissionregistrationv1.WebhookClientConfig{
			Service: &admissionregistrationv1.ServiceReference{
				Namespace: config.KyvernoNamespace(),
				Name:      config.KyvernoServiceName(),
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
		Name: name,
		Labels: map[string]string{
			managedByLabel: kyvernoValue,
		},
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

func constructPolicyValidatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.ValidatingWebhookConfiguration {
	name, path := config.PolicyValidatingWebhookName, config.PolicyValidatingWebhookServicePath
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyValidatingWebhookConfigurationName, owner),
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			generateValidatingWebhook(name, path, caData, timeoutSeconds, policyRule, createUpdate, admissionregistrationv1.Ignore),
		},
	}
}

func constructDebugPolicyValidatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.ValidatingWebhookConfiguration {
	name, url := config.PolicyValidatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.PolicyValidatingWebhookServicePath)
	return &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyValidatingWebhookConfigurationDebugName, owner),
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			generateDebugValidatingWebhook(name, url, caData, timeoutSeconds, policyRule, createUpdate, admissionregistrationv1.Ignore),
		},
	}
}

func constructPolicyMutatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, path := config.PolicyMutatingWebhookName, config.PolicyMutatingWebhookServicePath
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyMutatingWebhookConfigurationName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateMutatingWebhook(name, path, caData, timeoutSeconds, policyRule, createUpdate, admissionregistrationv1.Ignore),
		},
	}
}

func constructDebugPolicyMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, url := config.PolicyMutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.PolicyMutatingWebhookServicePath)
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.PolicyMutatingWebhookConfigurationDebugName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateDebugMutatingWebhook(name, url, caData, timeoutSeconds, policyRule, createUpdate, admissionregistrationv1.Ignore),
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

func defaultResourceWebhookRule(autoUpdate bool) admissionregistrationv1.Rule {
	if autoUpdate {
		return admissionregistrationv1.Rule{}
	}
	return admissionregistrationv1.Rule{
		APIGroups:   []string{"*"},
		APIVersions: []string{"*"},
		Resources:   []string{"*/*"},
	}
}

func constructDefaultDebugMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, url := config.MutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.MutatingWebhookServicePath)
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationDebugName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateDebugMutatingWebhook(name+"-ignore", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateDebugMutatingWebhook(name+"-fail", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Fail))
	}
	return webhook
}

func constructDefaultMutatingWebhookConfig(caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, path := config.MutatingWebhookName, config.MutatingWebhookServicePath
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateMutatingWebhook(name+"-ignore", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateMutatingWebhook(name+"-fail", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Fail))
	}
	return webhook
}

func constructDefaultDebugValidatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admissionregistrationv1.ValidatingWebhookConfiguration {
	name, url := config.ValidatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.ValidatingWebhookServicePath)
	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.ValidatingWebhookConfigurationDebugName, owner),
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			generateDebugValidatingWebhook(name+"-ignore", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateDebugValidatingWebhook(name+"-fail", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admissionregistrationv1.Fail))
	}
	return webhook
}

func constructDefaultValidatingWebhookConfig(caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admissionregistrationv1.ValidatingWebhookConfiguration {
	name, path := config.ValidatingWebhookName, config.ValidatingWebhookServicePath
	webhook := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.ValidatingWebhookConfigurationName, owner),
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			generateValidatingWebhook(name+"-ignore", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		webhook.Webhooks = append(webhook.Webhooks, generateValidatingWebhook(name+"-fail", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), all, admissionregistrationv1.Fail))
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

func constructVerifyMutatingWebhookConfig(caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, path := config.VerifyMutatingWebhookName, config.VerifyMutatingWebhookServicePath
	webhook := generateMutatingWebhook(name, path, caData, timeoutSeconds, verifyRule, update, admissionregistrationv1.Ignore)
	webhook.ObjectSelector = vertifyObjectSelector
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.VerifyMutatingWebhookConfigurationName, owner),
		Webhooks:   []admissionregistrationv1.MutatingWebhook{webhook},
	}
}

func constructDebugVerifyMutatingWebhookConfig(serverIP string, caData []byte, timeoutSeconds int32, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, url := config.VerifyMutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.VerifyMutatingWebhookServicePath)
	webhook := generateDebugMutatingWebhook(name, url, caData, timeoutSeconds, verifyRule, update, admissionregistrationv1.Ignore)
	webhook.ObjectSelector = vertifyObjectSelector
	return &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.VerifyMutatingWebhookConfigurationDebugName, owner),
		Webhooks:   []admissionregistrationv1.MutatingWebhook{webhook},
	}
}
