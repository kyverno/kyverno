package config

import (
	"context"
	"strconv"
	"sync"

	valid "github.com/asaskevich/govalidator"
	osutils "github.com/kyverno/kyverno/pkg/utils/os"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	// GetDefaultRegistry return default image registry
	GetDefaultRegistry() string
	// GetEnableDefaultRegistryMutation return if should mutate image registry
	GetEnableDefaultRegistryMutation() bool
	// ToFilter checks if the given resource is set to be filtered in the configuration
	ToFilter(kind schema.GroupVersionKind, subresource, namespace, name string) bool
	// GetExcludedGroups return excluded groups
	GetExcludedGroups() []string
	// GetExcludedUsernames return excluded usernames
	GetExcludedUsernames() []string
	// GetExcludedRoles return excluded roles
	GetExcludedRoles() []string
	// GetExcludedClusterRoles return excluded roles
	GetExcludedClusterRoles() []string
	// GetGenerateSuccessEvents return if should generate success events
	GetGenerateSuccessEvents() bool
	// GetWebhooks returns the webhook configs
	GetWebhooks() []WebhookConfig
	// GetWebhookAnnotations returns annotations to set on webhook configs
	GetWebhookAnnotations() map[string]string
	// Load loads configuration from a configmap
	Load(cm *corev1.ConfigMap)
}

// configuration stores the configuration
type configuration struct {
	skipResourceFilters           bool
	defaultRegistry               string
	enableDefaultRegistryMutation bool
	excludedGroups                []string
	excludedUsernames             []string
	excludedRoles                 []string
	excludedClusterRoles          []string
	filters                       []filter
	generateSuccessEvents         bool
	webhooks                      []WebhookConfig
	webhookAnnotations            map[string]string
	mux                           sync.RWMutex
}

// NewDefaultConfiguration ...
func NewDefaultConfiguration(skipResourceFilters bool) *configuration {
	return &configuration{
		skipResourceFilters:           skipResourceFilters,
		defaultRegistry:               "docker.io",
		enableDefaultRegistryMutation: true,
	}
}

// NewConfiguration ...
func NewConfiguration(client kubernetes.Interface, skipResourceFilters bool) (Configuration, error) {
	cd := NewDefaultConfiguration(skipResourceFilters)
	if cm, err := client.CoreV1().ConfigMaps(kyvernoNamespace).Get(context.TODO(), kyvernoConfigMapName, metav1.GetOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		cd.load(cm)
	}
	return cd, nil
}

func (cd *configuration) ToFilter(gvk schema.GroupVersionKind, subresource, namespace, name string) bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	if !cd.skipResourceFilters {
		for _, f := range cd.filters {
			if wildcard.Match(f.Group, gvk.Group) &&
				wildcard.Match(f.Version, gvk.Version) &&
				wildcard.Match(f.Kind, gvk.Kind) &&
				wildcard.Match(f.Subresource, subresource) &&
				wildcard.Match(f.Namespace, namespace) &&
				wildcard.Match(f.Name, name) {
				return true
			}
			// [Namespace,kube-system,*] || [*,kube-system,*]
			if gvk.Group == "" && gvk.Version == "v1" && gvk.Kind == "Namespace" {
				if wildcard.Match(f.Group, gvk.Group) &&
					wildcard.Match(f.Version, gvk.Version) &&
					wildcard.Match(f.Kind, gvk.Kind) &&
					wildcard.Match(f.Namespace, name) {
					return true
				}
				if gvk.Group == "" && gvk.Version == "v1" && gvk.Kind == "Namespace" {
					if wildcard.Match(f.Group, gvk.Group) &&
						wildcard.Match(f.Version, gvk.Version) &&
						wildcard.Match(f.Kind, gvk.Kind) &&
						wildcard.Match(f.Namespace, name) {
						return true
					}
				}
			}
		}
	}
	return false
}

func (cd *configuration) GetDefaultRegistry() string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.defaultRegistry
}

func (cd *configuration) GetEnableDefaultRegistryMutation() bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.enableDefaultRegistryMutation
}

func (cd *configuration) GetExcludedUsernames() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludedUsernames
}

func (cd *configuration) GetExcludedRoles() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludedRoles
}

func (cd *configuration) GetExcludedClusterRoles() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludedClusterRoles
}

func (cd *configuration) GetExcludedGroups() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludedGroups
}

func (cd *configuration) GetGenerateSuccessEvents() bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.generateSuccessEvents
}

func (cd *configuration) GetWebhooks() []WebhookConfig {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.webhooks
}

func (cd *configuration) GetWebhookAnnotations() map[string]string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.webhookAnnotations
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
	cd.excludedUsernames = []string{}
	cd.excludedGroups = []string{}
	cd.excludedRoles = []string{}
	cd.excludedClusterRoles = []string{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	// load filters
	cd.filters = parseKinds(cm.Data["resourceFilters"])
	newDefaultRegistry, ok := cm.Data["defaultRegistry"]
	if !ok {
		logger.V(6).Info("configuration: No defaultRegistry defined in ConfigMap")
	} else {
		if valid.IsDNSName(newDefaultRegistry) {
			logger.V(4).Info("Updated defaultRegistry config parameter.", "oldDefaultRegistry", cd.defaultRegistry, "newDefaultRegistry", newDefaultRegistry)
			cd.defaultRegistry = newDefaultRegistry
		} else {
			logger.V(4).Info("defaultRegistry didn't change because the provided config value isn't a valid DNS hostname")
		}
	}
	enableDefaultRegistryMutation, ok := cm.Data["enableDefaultRegistryMutation"]
	if !ok {
		logger.V(6).Info("configuration: No enableDefaultRegistryMutation defined in ConfigMap")
	} else {
		newEnableDefaultRegistryMutation, err := strconv.ParseBool(enableDefaultRegistryMutation)
		if err != nil {
			logger.V(4).Info("configuration: Invalid value for enableDefaultRegistryMutation defined in ConfigMap. enableDefaultRegistryMutation didn't change")
		}
		logger.V(4).Info("Updated enableDefaultRegistryMutation config parameter", "oldEnableDefaultRegistryMutation", cd.enableDefaultRegistryMutation, "newEnableDefaultRegistryMutation", newEnableDefaultRegistryMutation)
		cd.enableDefaultRegistryMutation = newEnableDefaultRegistryMutation
	}
	// load excludeGroupRole
	excludedGroups, ok := cm.Data["excludeGroups"]
	if !ok {
		logger.V(6).Info("configuration: No excludeGroups defined in ConfigMap")
	} else {
		cd.excludedGroups = parseStrings(excludedGroups)
	}
	// load excludeUsername
	excludedUsernames, ok := cm.Data["excludeUsernames"]
	if !ok {
		logger.V(6).Info("configuration: No excludeUsernames defined in ConfigMap")
	} else {
		cd.excludedUsernames = parseStrings(excludedUsernames)
	}
	// load excludeRoles
	excludedRoles, ok := cm.Data["excludeRoles"]
	if !ok {
		logger.V(6).Info("configuration: No excludeRoles defined in ConfigMap")
	} else {
		cd.excludedRoles = parseStrings(excludedRoles)
	}
	// load excludeClusterRoles
	excludedClusterRoles, ok := cm.Data["excludeClusterRoles"]
	if !ok {
		logger.V(6).Info("configuration: No excludeClusterRoles defined in ConfigMap")
	} else {
		cd.excludedClusterRoles = parseStrings(excludedClusterRoles)
	}
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
	// load webhook annotations
	webhookAnnotations, ok := cm.Data["webhookAnnotations"]
	if ok {
		webhookAnnotations, err := parseWebhookAnnotations(webhookAnnotations)
		if err != nil {
			logger.Error(err, "failed to parse webhook annotations")
		} else {
			cd.webhookAnnotations = webhookAnnotations
		}
	}
}

func (cd *configuration) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []filter{}
	cd.defaultRegistry = "docker.io"
	cd.enableDefaultRegistryMutation = true
	cd.excludedUsernames = []string{}
	cd.excludedGroups = []string{}
	cd.excludedRoles = []string{}
	cd.excludedClusterRoles = []string{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.webhookAnnotations = nil
}
