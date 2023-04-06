package config

import (
	"context"
	"errors"
	"strconv"
	"sync"

	valid "github.com/asaskevich/govalidator"
	osutils "github.com/kyverno/kyverno/pkg/utils/os"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		if !apierrors.IsNotFound(err) {
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
	cd.defaultRegistry = "docker.io"
	cd.enableDefaultRegistryMutation = true
	cd.excludedUsernames = []string{}
	cd.excludedGroups = []string{}
	cd.excludedRoles = []string{}
	cd.excludedClusterRoles = []string{}
	cd.filters = []filter{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.webhookAnnotations = nil
	// load filters
	cd.filters = parseKinds(cm.Data["resourceFilters"])
	logger.Info("filters configured", "filters", cd.filters)
	// load defaultRegistry
	defaultRegistry, ok := cm.Data["defaultRegistry"]
	if !ok {
		logger.Info("defaultRegistry not set")
	} else {
		logger := logger.WithValues("defaultRegistry", defaultRegistry)
		if valid.IsDNSName(defaultRegistry) {
			cd.defaultRegistry = defaultRegistry
			logger.Info("defaultRegistry configured")
		} else {
			logger.Error(errors.New("defaultRegistry is not a valid DNS hostname"), "failed to configure defaultRegistry")
		}
	}
	// load enableDefaultRegistryMutation
	enableDefaultRegistryMutation, ok := cm.Data["enableDefaultRegistryMutation"]
	if !ok {
		logger.Info("enableDefaultRegistryMutation not set")
	} else {
		logger := logger.WithValues("enableDefaultRegistryMutation", enableDefaultRegistryMutation)
		enableDefaultRegistryMutation, err := strconv.ParseBool(enableDefaultRegistryMutation)
		if err != nil {
			logger.Error(err, "enableDefaultRegistryMutation is not a boolean")
		} else {
			cd.enableDefaultRegistryMutation = enableDefaultRegistryMutation
			logger.Info("enableDefaultRegistryMutation configured")
		}
	}
	// load excludeGroupRole
	excludedGroups, ok := cm.Data["excludeGroups"]
	if !ok {
		logger.Info("excludeGroups not set")
	} else {
		cd.excludedGroups = parseStrings(excludedGroups)
		logger.Info("excludedGroups configured", "excludeGroups", cd.excludedGroups)
	}
	// load excludeUsername
	excludedUsernames, ok := cm.Data["excludeUsernames"]
	if !ok {
		logger.Info("excludeUsernames not set")
	} else {
		cd.excludedUsernames = parseStrings(excludedUsernames)
		logger.Info("excludedUsernames configured", "excludeUsernames", cd.excludedUsernames)
	}
	// load excludeRoles
	excludedRoles, ok := cm.Data["excludeRoles"]
	if !ok {
		logger.Info("excludeRoles not set")
	} else {
		cd.excludedRoles = parseStrings(excludedRoles)
		logger.Info("excludedRoles configured", "excludeRoles", cd.excludedRoles)
	}
	// load excludeClusterRoles
	excludedClusterRoles, ok := cm.Data["excludeClusterRoles"]
	if !ok {
		logger.Info("excludeClusterRoles not set")
	} else {
		cd.excludedClusterRoles = parseStrings(excludedClusterRoles)
		logger.Info("excludedClusterRoles configured", "excludeClusterRoles", cd.excludedClusterRoles)
	}
	// load generateSuccessEvents
	generateSuccessEvents, ok := cm.Data["generateSuccessEvents"]
	if !ok {
		logger.Info("generateSuccessEvents not set")
	} else {
		logger := logger.WithValues("generateSuccessEvents", generateSuccessEvents)
		generateSuccessEvents, err := strconv.ParseBool(generateSuccessEvents)
		if err != nil {
			logger.Error(err, "generateSuccessEvents is not a boolean")
		} else {
			cd.generateSuccessEvents = generateSuccessEvents
			logger.Info("generateSuccessEvents configured")
		}
	}
	// load webhooks
	webhooks, ok := cm.Data["webhooks"]
	if !ok {
		logger.Info("webhooks not set")
	} else {
		logger := logger.WithValues("webhooks", webhooks)
		webhooks, err := parseWebhooks(webhooks)
		if err != nil {
			logger.Error(err, "failed to parse webhooks")
		} else {
			cd.webhooks = webhooks
			logger.Info("webhooks configured")
		}
	}
	// load webhook annotations
	webhookAnnotations, ok := cm.Data["webhookAnnotations"]
	if !ok {
		logger.Info("webhookAnnotations not set")
	} else {
		logger := logger.WithValues("webhookAnnotations", webhookAnnotations)
		webhookAnnotations, err := parseWebhookAnnotations(webhookAnnotations)
		if err != nil {
			logger.Error(err, "failed to parse webhook annotations")
		} else {
			cd.webhookAnnotations = webhookAnnotations
			logger.Info("webhookAnnotations configured")
		}
	}
}

func (cd *configuration) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.defaultRegistry = "docker.io"
	cd.enableDefaultRegistryMutation = true
	cd.excludedUsernames = []string{}
	cd.excludedGroups = []string{}
	cd.excludedRoles = []string{}
	cd.excludedClusterRoles = []string{}
	cd.filters = []filter{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.webhookAnnotations = nil
}
