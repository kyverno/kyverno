package config

import (
	"context"
	"strconv"
	"sync"

	osutils "github.com/kyverno/kyverno/pkg/utils/os"
	wildcard "github.com/kyverno/kyverno/pkg/utils/wildcard"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml

// webhook configuration names
const (
	// PolicyValidatingWebhookConfigurationName default policy validating webhook configuration name
	PolicyValidatingWebhookConfigurationName = "kyverno-policy-validating-webhook-cfg"
	// ValidatingWebhookConfigurationName ...
	ValidatingWebhookConfigurationName = "kyverno-resource-validating-webhook-cfg"
	// ExceptionValidatingWebhookConfigurationName ...
	ExceptionValidatingWebhookConfigurationName = "kyverno-exception-validating-webhook-cfg"
	// CleanupValidatingWebhookConfigurationName ...
	CleanupValidatingWebhookConfigurationName = "kyverno-cleanup-validating-webhook-cfg"
	// PolicyMutatingWebhookConfigurationName default policy mutating webhook configuration name
	PolicyMutatingWebhookConfigurationName = "kyverno-policy-mutating-webhook-cfg"
	// MutatingWebhookConfigurationName default resource mutating webhook configuration name
	MutatingWebhookConfigurationName = "kyverno-resource-mutating-webhook-cfg"
	// VerifyMutatingWebhookConfigurationName default verify mutating webhook configuration name
	VerifyMutatingWebhookConfigurationName = "kyverno-verify-mutating-webhook-cfg"
)

// webhook names
const (
	// PolicyValidatingWebhookName default policy validating webhook name
	PolicyValidatingWebhookName = "validate-policy.kyverno.svc"
	// ValidatingWebhookName ...
	ValidatingWebhookName = "validate.kyverno.svc"
	// PolicyMutatingWebhookName default policy mutating webhook name
	PolicyMutatingWebhookName = "mutate-policy.kyverno.svc"
	// MutatingWebhookName default resource mutating webhook name
	MutatingWebhookName = "mutate.kyverno.svc"
	// VerifyMutatingWebhookName default verify mutating webhook name
	VerifyMutatingWebhookName = "monitor-webhooks.kyverno.svc"
)

// paths
const (
	// PolicyValidatingWebhookServicePath is the path for policy validation webhook(used to validate policy resource)
	PolicyValidatingWebhookServicePath = "/policyvalidate"
	// ValidatingWebhookServicePath is the path for validation webhook
	ValidatingWebhookServicePath = "/validate"
	// ExceptionValidatingWebhookServicePath is the path for policy exception validation webhook(used to validate policy exception resource)
	ExceptionValidatingWebhookServicePath = "/exceptionvalidate"
	// CleanupValidatingWebhookServicePath is the path for cleanup policy validation webhook(used to validate cleanup policy resource)
	CleanupValidatingWebhookServicePath = "/validate"
	// PolicyMutatingWebhookServicePath is the path for policy mutation webhook(used to default)
	PolicyMutatingWebhookServicePath = "/policymutate"
	// MutatingWebhookServicePath is the path for mutation webhook
	MutatingWebhookServicePath = "/mutate"
	// VerifyMutatingWebhookServicePath is the path for verify webhook(used to veryfing if admission control is enabled and active)
	VerifyMutatingWebhookServicePath = "/verifymutate"
	// LivenessServicePath is the path for check liveness health
	LivenessServicePath = "/health/liveness"
	// ReadinessServicePath is the path for check readness health
	ReadinessServicePath = "/health/readiness"
	// MetricsPath is the path for exposing metrics
	MetricsPath = "/metrics"
)

const (
	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants
	// ClusterRoleAPIVersion define the default clusterrole resource apiVersion
	ClusterRoleAPIVersion = "rbac.authorization.k8s.io/v1"
	// ClusterRoleKind define the default clusterrole resource kind
	ClusterRoleKind = "ClusterRole"
)

var (
	// kyvernoNamespace is the Kyverno namespace
	kyvernoNamespace = osutils.GetEnvWithFallback("KYVERNO_NAMESPACE", "kyverno")
	// kyvernoServiceAccountName is the Kyverno service account name
	kyvernoServiceAccountName = osutils.GetEnvWithFallback("KYVERNO_SERVICEACCOUNT_NAME", "kyverno")
	// kyvernoDeploymentName is the Kyverno deployment name
	kyvernoDeploymentName = osutils.GetEnvWithFallback("KYVERNO_DEPLOYMENT", "kyverno")
	// kyvernoServiceName is the Kyverno service name
	kyvernoServiceName = osutils.GetEnvWithFallback("KYVERNO_SVC", "kyverno-svc")
	// kyvernoPodName is the Kyverno pod name
	kyvernoPodName = osutils.GetEnvWithFallback("KYVERNO_POD_NAME", "kyverno")
	// kyvernoConfigMapName is the Kyverno configmap name
	kyvernoConfigMapName = osutils.GetEnvWithFallback("INIT_CONFIG", "kyverno")
	// defaultExcludeGroupRole ...
	defaultExcludeGroupRole []string = []string{"system:serviceaccounts:kube-system", "system:nodes", "system:kube-scheduler"}
	// kyvernoDryRunNamespace is the namespace for DryRun option of YAML verification
	kyvernoDryrunNamespace = osutils.GetEnvWithFallback("KYVERNO_DRYRUN_NAMESPACE", "kyverno-dryrun")
)

func KyvernoNamespace() string {
	return kyvernoNamespace
}

func KyvernoDryRunNamespace() string {
	return kyvernoDryrunNamespace
}

func KyvernoServiceAccountName() string {
	return kyvernoServiceAccountName
}

func KyvernoDeploymentName() string {
	return kyvernoDeploymentName
}

func KyvernoServiceName() string {
	return kyvernoServiceName
}

func KyvernoPodName() string {
	return kyvernoPodName
}

func KyvernoConfigMapName() string {
	return kyvernoConfigMapName
}

// Configuration to be used by consumer to check filters
type Configuration interface {
	// ToFilter checks if the given resource is set to be filtered in the configuration
	ToFilter(kind, namespace, name string) bool
	// GetExcludeGroupRole return exclude roles
	GetExcludeGroupRole() []string
	// GetExcludeUsername return exclude username
	GetExcludeUsername() []string
	// GetGenerateSuccessEvents return if should generate success events
	GetGenerateSuccessEvents() bool
	// FilterNamespaces filters exclude namespace
	FilterNamespaces(namespaces []string) []string
	// GetWebhooks returns the webhook configs
	GetWebhooks() []WebhookConfig
	// Load loads configuration from a configmap
	Load(cm *corev1.ConfigMap)
}

// configuration stores the configuration
type configuration struct {
	mux                   sync.RWMutex
	filters               []filter
	excludeGroupRole      []string
	excludeUsername       []string
	webhooks              []WebhookConfig
	generateSuccessEvents bool
}

// NewDefaultConfiguration ...
func NewDefaultConfiguration() *configuration {
	return &configuration{
		excludeGroupRole: defaultExcludeGroupRole,
	}
}

// NewConfiguration ...
func NewConfiguration(client kubernetes.Interface) (Configuration, error) {
	cd := NewDefaultConfiguration()
	if cm, err := client.CoreV1().ConfigMaps(kyvernoNamespace).Get(context.TODO(), kyvernoConfigMapName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		cd.load(cm)
	}
	return cd, nil
}

func (cd *configuration) ToFilter(kind, namespace, name string) bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	for _, f := range cd.filters {
		if wildcard.Match(f.Kind, kind) && wildcard.Match(f.Namespace, namespace) && wildcard.Match(f.Name, name) {
			return true
		}
		if kind == "Namespace" {
			// [Namespace,kube-system,*] || [*,kube-system,*]
			if (f.Kind == "Namespace" || f.Kind == "*") && wildcard.Match(f.Namespace, name) {
				return true
			}
		}
	}
	return false
}

func (cd *configuration) GetExcludeGroupRole() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeGroupRole
}

func (cd *configuration) GetExcludeUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeUsername
}

func (cd *configuration) GetGenerateSuccessEvents() bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.generateSuccessEvents
}

func (cd *configuration) FilterNamespaces(namespaces []string) []string {
	var results []string
	for _, ns := range namespaces {
		if !cd.ToFilter("", ns, "") {
			results = append(results, ns)
		}
	}
	return results
}

func (cd *configuration) GetWebhooks() []WebhookConfig {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.webhooks
}

func (cd *configuration) Load(cm *corev1.ConfigMap) {
	if cm != nil {
		cd.load(cm)
	} else {
		cd.unload()
	}
}

func (cd *configuration) load(cm *corev1.ConfigMap) {
	logger := logger.WithValues("name", cm.Name, "namespace", cm.Namespace)
	if cm.Data == nil {
		return
	}
	cd.mux.Lock()
	defer cd.mux.Unlock()
	// reset
	cd.filters = []filter{}
	cd.excludeGroupRole = []string{}
	cd.excludeUsername = []string{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	// load filters
	cd.filters = parseKinds(cm.Data["resourceFilters"])
	// load excludeGroupRole
	cd.excludeGroupRole = append(cd.excludeGroupRole, parseRbac(cm.Data["excludeGroupRole"])...)
	cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	// load excludeUsername
	cd.excludeUsername = append(cd.excludeUsername, parseRbac(cm.Data["excludeUsername"])...)
	// load generateSuccessEvents
	generateSuccessEvents, ok := cm.Data["generateSuccessEvents"]
	if ok {
		generateSuccessEvents, err := strconv.ParseBool(generateSuccessEvents)
		if err != nil {
			logger.Error(err, "failed to parse generateSuccessEvents")
		} else {
			cd.generateSuccessEvents = generateSuccessEvents
		}
	}
	// load webhooks
	webhooks, ok := cm.Data["webhooks"]
	if ok {
		webhooks, err := parseWebhooks(webhooks)
		if err != nil {
			logger.Error(err, "failed to parse webhooks")
		} else {
			cd.webhooks = webhooks
		}
	}
}

func (cd *configuration) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []filter{}
	cd.excludeGroupRole = []string{}
	cd.excludeUsername = []string{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
}
