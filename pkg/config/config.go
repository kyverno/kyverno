package config

import (
	"errors"
	"fmt"
	"strconv"
	"sync"

	valid "github.com/asaskevich/govalidator"
	osutils "github.com/kyverno/kyverno/pkg/utils/os"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	// TtlValidatingWebhookConfigurationName ttl label validating webhook configuration name
	TtlValidatingWebhookConfigurationName = "kyverno-ttl-validating-webhook-cfg"
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
	// TtlValidatingWebhookServicePath is the path for validation of cleanup.kyverno.io/ttl label value
	TtlValidatingWebhookServicePath = "/verifyttl"
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

// keys in config map
const (
	resourceFilters               = "resourceFilters"
	defaultRegistry               = "defaultRegistry"
	enableDefaultRegistryMutation = "enableDefaultRegistryMutation"
	excludeGroups                 = "excludeGroups"
	excludeUsernames              = "excludeUsernames"
	excludeRoles                  = "excludeRoles"
	excludeClusterRoles           = "excludeClusterRoles"
	generateSuccessEvents         = "generateSuccessEvents"
	webhooks                      = "webhooks"
	webhookAnnotations            = "webhookAnnotations"
	matchConditions               = "matchConditions"
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
	// kyvernoMetricsConfigMapName is the Kyverno metrics configmap name
	kyvernoMetricsConfigMapName = osutils.GetEnvWithFallback("METRICS_CONFIG", "kyverno-metrics")
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

func KyvernoMetricsConfigMapName() string {
	return kyvernoMetricsConfigMapName
}

func KyvernoUserName(serviceaccount string) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", kyvernoNamespace, serviceaccount)
}

// Configuration to be used by consumer to check filters
type Configuration interface {
	// GetDefaultRegistry return default image registry
	GetDefaultRegistry() string
	// GetEnableDefaultRegistryMutation returns true if image references should be mutated
	GetEnableDefaultRegistryMutation() bool
	// IsExcluded checks exlusions/inclusions to determine if the admission request should be excluded or not
	IsExcluded(username string, groups []string, roles []string, clusterroles []string) bool
	// ToFilter checks if the given resource is set to be filtered in the configuration
	ToFilter(kind schema.GroupVersionKind, subresource, namespace, name string) bool
	// GetGenerateSuccessEvents return if should generate success events
	GetGenerateSuccessEvents() bool
	// GetWebhooks returns the webhook configs
	GetWebhooks() []WebhookConfig
	// GetWebhookAnnotations returns annotations to set on webhook configs
	GetWebhookAnnotations() map[string]string
	// GetMatchConditions returns match conditions to set on webhook configs
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	// Load loads configuration from a configmap
	Load(*corev1.ConfigMap)
	// OnChanged adds a callback to be invoked when the configuration is reloaded
	OnChanged(func())
}

// configuration stores the configuration
type configuration struct {
	skipResourceFilters           bool
	defaultRegistry               string
	enableDefaultRegistryMutation bool
	exclusions                    match
	inclusions                    match
	filters                       []filter
	generateSuccessEvents         bool
	webhooks                      []WebhookConfig
	webhookAnnotations            map[string]string
	matchConditions               []admissionregistrationv1.MatchCondition
	mux                           sync.RWMutex
	callbacks                     []func()
}

type match struct {
	groups       []string
	usernames    []string
	roles        []string
	clusterroles []string
}

func (c match) matches(username string, groups []string, roles []string, clusterroles []string) bool {
	// filter by username
	for _, pattern := range c.usernames {
		if wildcard.Match(pattern, username) {
			return true
		}
	}
	// filter by groups
	for _, pattern := range c.groups {
		for _, candidate := range groups {
			if wildcard.Match(pattern, candidate) {
				return true
			}
		}
	}
	// filter by roles
	for _, pattern := range c.roles {
		for _, candidate := range roles {
			if wildcard.Match(pattern, candidate) {
				return true
			}
		}
	}
	// filter by cluster roles
	for _, pattern := range c.clusterroles {
		for _, candidate := range clusterroles {
			if wildcard.Match(pattern, candidate) {
				return true
			}
		}
	}
	return false
}

// NewDefaultConfiguration ...
func NewDefaultConfiguration(skipResourceFilters bool) *configuration {
	return &configuration{
		skipResourceFilters:           skipResourceFilters,
		defaultRegistry:               "docker.io",
		enableDefaultRegistryMutation: true,
	}
}

func (cd *configuration) OnChanged(callback func()) {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.callbacks = append(cd.callbacks, callback)
}

func (c *configuration) IsExcluded(username string, groups []string, roles []string, clusterroles []string) bool {
	if c.inclusions.matches(username, groups, roles, clusterroles) {
		return false
	}
	return c.exclusions.matches(username, groups, roles, clusterroles)
}

func (cd *configuration) ToFilter(gvk schema.GroupVersionKind, subresource, namespace, name string) bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	if !cd.skipResourceFilters {
		for _, f := range cd.filters {
			if wildcard.Match(f.Group, gvk.Group) && wildcard.Match(f.Version, gvk.Version) && wildcard.Match(f.Kind, gvk.Kind) && wildcard.Match(f.Subresource, subresource) {
				if wildcard.Match(f.Namespace, namespace) && wildcard.Match(f.Name, name) {
					return true
				}
				// [Namespace,kube-system,*] || [*,kube-system,*]
				if gvk.Group == "" && gvk.Version == "v1" && gvk.Kind == "Namespace" {
					if wildcard.Match(f.Namespace, name) {
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

func (cd *configuration) GetMatchConditions() []admissionregistrationv1.MatchCondition {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.matchConditions
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
	cd.mux.Lock()
	defer cd.mux.Unlock()
	defer cd.notify()
	data := cm.Data
	if data == nil {
		data = map[string]string{}
	}
	// reset
	cd.defaultRegistry = "docker.io"
	cd.enableDefaultRegistryMutation = true
	cd.exclusions = match{}
	cd.inclusions = match{}
	cd.filters = []filter{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.webhookAnnotations = nil
	cd.matchConditions = nil
	// load filters
	cd.filters = parseKinds(data[resourceFilters])
	logger.Info("filters configured", "filters", cd.filters)
	// load defaultRegistry
	defaultRegistry, ok := data[defaultRegistry]
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
	enableDefaultRegistryMutation, ok := data[enableDefaultRegistryMutation]
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
	excludedGroups, ok := data[excludeGroups]
	if !ok {
		logger.Info("excludeGroups not set")
	} else {
		cd.exclusions.groups, cd.inclusions.groups = parseExclusions(excludedGroups)
		logger.Info("excludedGroups configured", "excludeGroups", cd.exclusions.groups, "includeGroups", cd.inclusions.groups)
	}
	// load excludeUsername
	excludedUsernames, ok := data[excludeUsernames]
	if !ok {
		logger.Info("excludeUsernames not set")
	} else {
		cd.exclusions.usernames, cd.inclusions.usernames = parseExclusions(excludedUsernames)
		logger.Info("excludedUsernames configured", "excludeUsernames", cd.exclusions.usernames, "includeUsernames", cd.inclusions.usernames)
	}
	// load excludeRoles
	excludedRoles, ok := data[excludeRoles]
	if !ok {
		logger.Info("excludeRoles not set")
	} else {
		cd.exclusions.roles, cd.inclusions.roles = parseExclusions(excludedRoles)
		logger.Info("excludedRoles configured", "excludeRoles", cd.exclusions.roles, "includeRoles", cd.inclusions.roles)
	}
	// load excludeClusterRoles
	excludedClusterRoles, ok := data[excludeClusterRoles]
	if !ok {
		logger.Info("excludeClusterRoles not set")
	} else {
		cd.exclusions.clusterroles, cd.inclusions.clusterroles = parseExclusions(excludedClusterRoles)
		logger.Info("excludedClusterRoles configured", "excludeClusterRoles", cd.exclusions.clusterroles, "includeClusterRoles", cd.inclusions.clusterroles)
	}
	// load generateSuccessEvents
	generateSuccessEvents, ok := data[generateSuccessEvents]
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
	webhooks, ok := data[webhooks]
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
	webhookAnnotations, ok := data[webhookAnnotations]
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
	// load match conditions
	matchConditions, ok := data[matchConditions]
	if !ok {
		logger.Info("matchConditions not set")
	} else {
		logger := logger.WithValues("matchConditions", matchConditions)
		matchConditions, err := parseMatchConditions(matchConditions)
		if err != nil {
			logger.Error(err, "failed to parse match conditions")
		} else {
			cd.matchConditions = matchConditions
			logger.Info("matchConditions configured")
		}
	}
}

func (cd *configuration) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	defer cd.notify()
	cd.defaultRegistry = "docker.io"
	cd.enableDefaultRegistryMutation = true
	cd.exclusions = match{}
	cd.inclusions = match{}
	cd.filters = []filter{}
	cd.generateSuccessEvents = false
	cd.webhooks = nil
	cd.webhookAnnotations = nil
	logger.Info("configuration unloaded")
}

func (cd *configuration) notify() {
	for _, callback := range cd.callbacks {
		callback()
	}
}
