package webhookconfig

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tls"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	managedByLabel string = "webhook.kyverno.io/managed-by"
)

var (
	noneOnDryRun = admissionregistrationv1.SideEffectClassNoneOnDryRun
	never        = admissionregistrationv1.NeverReinvocationPolicy
	ifNeeded     = admissionregistrationv1.IfNeededReinvocationPolicy
	createUpdate = []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update}
)

func (wrc *Register) readCaData() []byte {
	logger := wrc.log.WithName("readCaData")
	var caData []byte
	var err error
	recorder := metrics.NamespacedClientQueryRecorder(wrc.metricsConfig, config.KyvernoNamespace(), "Secret", metrics.KubeClient)
	secretsClient := metrics.ObjectClient[*corev1.Secret](recorder, wrc.kubeClient.CoreV1().Secrets(config.KyvernoNamespace()))
	// Check if ca is defined in the secret tls-ca
	// assume the key and signed cert have been defined in secret tls.kyverno
	if caData, err = tls.ReadRootCASecret(secretsClient); err == nil {
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
			"app.kubernetes.io/name": kyvernov1.ValueKyvernoApp,
		},
	}
	clusterRoles, err := wrc.kubeClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(selector)})
	wrc.metricsConfig.RecordClientQueries(metrics.ClientList, metrics.KubeClient, "ClusterRole", "")
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

func generateObjectMeta(name string, owner ...metav1.OwnerReference) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: name,
		Labels: map[string]string{
			managedByLabel: kyvernov1.ValueKyvernoApp,
		},
		OwnerReferences: owner,
	}
}

// resource webhook configuration utils

func getResourceMutatingWebhookConfigName(serverIP string) string {
	if serverIP != "" {
		return config.MutatingWebhookConfigurationDebugName
	}
	return config.MutatingWebhookConfigurationName
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
	name, baseUrl := config.MutatingWebhookName, fmt.Sprintf("https://%s%s", serverIP, config.MutatingWebhookServicePath)
	url := fmt.Sprintf("%s/ignore", baseUrl)
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationDebugName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateDebugMutatingWebhook(name+"-ignore", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		url := fmt.Sprintf("%s/fail", baseUrl)
		webhook.Webhooks = append(webhook.Webhooks, generateDebugMutatingWebhook(name+"-fail", url, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Fail))
	}
	return webhook
}

func constructDefaultMutatingWebhookConfig(caData []byte, timeoutSeconds int32, autoUpdate bool, owner metav1.OwnerReference) *admissionregistrationv1.MutatingWebhookConfiguration {
	name, basePath := config.MutatingWebhookName, config.MutatingWebhookServicePath
	path := fmt.Sprintf("%s/ignore", basePath)
	webhook := &admissionregistrationv1.MutatingWebhookConfiguration{
		ObjectMeta: generateObjectMeta(config.MutatingWebhookConfigurationName, owner),
		Webhooks: []admissionregistrationv1.MutatingWebhook{
			generateMutatingWebhook(name+"-ignore", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Ignore),
		},
	}
	if autoUpdate {
		path := fmt.Sprintf("%s/fail", basePath)
		webhook.Webhooks = append(webhook.Webhooks, generateMutatingWebhook(name+"-fail", path, caData, timeoutSeconds, defaultResourceWebhookRule(autoUpdate), createUpdate, admissionregistrationv1.Fail))
	}
	return webhook
}
