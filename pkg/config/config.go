package config

import (
	"context"
	"reflect"
	"strconv"
	"sync"

	wildcard "github.com/kyverno/go-wildcard"
	osutils "github.com/kyverno/kyverno/pkg/utils/os"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// These constants MUST be equal to the corresponding names in service definition in definitions/install.yaml
const (
	// MutatingWebhookConfigurationName default resource mutating webhook configuration name
	MutatingWebhookConfigurationName = "kyverno-resource-mutating-webhook-cfg"
	// MutatingWebhookConfigurationDebugName default resource mutating webhook configuration name for debug mode
	MutatingWebhookConfigurationDebugName = "kyverno-resource-mutating-webhook-cfg-debug"
	// MutatingWebhookName default resource mutating webhook name
	MutatingWebhookName = "mutate.kyverno.svc"
	// ValidatingWebhookConfigurationName ...
	ValidatingWebhookConfigurationName = "kyverno-resource-validating-webhook-cfg"
	// ValidatingWebhookConfigurationDebugName ...
	ValidatingWebhookConfigurationDebugName = "kyverno-resource-validating-webhook-cfg-debug"
	// ValidatingWebhookName ...
	ValidatingWebhookName = "validate.kyverno.svc"
	//VerifyMutatingWebhookConfigurationName default verify mutating webhook configuration name
	VerifyMutatingWebhookConfigurationName = "kyverno-verify-mutating-webhook-cfg"
	//VerifyMutatingWebhookConfigurationDebugName default verify mutating webhook configuration name for debug mode
	VerifyMutatingWebhookConfigurationDebugName = "kyverno-verify-mutating-webhook-cfg-debug"
	//VerifyMutatingWebhookName default verify mutating webhook name
	VerifyMutatingWebhookName = "monitor-webhooks.kyverno.svc"
	//PolicyValidatingWebhookConfigurationName default policy validating webhook configuration name
	PolicyValidatingWebhookConfigurationName = "kyverno-policy-validating-webhook-cfg"
	//PolicyValidatingWebhookConfigurationDebugName default policy validating webhook configuration name for debug mode
	PolicyValidatingWebhookConfigurationDebugName = "kyverno-policy-validating-webhook-cfg-debug"
	//PolicyValidatingWebhookName default policy validating webhook name
	PolicyValidatingWebhookName = "validate-policy.kyverno.svc"
	//PolicyMutatingWebhookConfigurationName default policy mutating webhook configuration name
	PolicyMutatingWebhookConfigurationName = "kyverno-policy-mutating-webhook-cfg"
	//PolicyMutatingWebhookConfigurationDebugName default policy mutating webhook configuration name for debug mode
	PolicyMutatingWebhookConfigurationDebugName = "kyverno-policy-mutating-webhook-cfg-debug"
	//PolicyMutatingWebhookName default policy mutating webhook name
	PolicyMutatingWebhookName = "mutate-policy.kyverno.svc"
	// Due to kubernetes issue, we must use next literal constants instead of deployment TypeMeta fields
	// Issue: https://github.com/kubernetes/kubernetes/pull/63972
	// When the issue is closed, we should use TypeMeta struct instead of this constants
	// ClusterRoleAPIVersion define the default clusterrole resource apiVersion
	ClusterRoleAPIVersion = "rbac.authorization.k8s.io/v1"
	// ClusterRoleKind define the default clusterrole resource kind
	ClusterRoleKind = "ClusterRole"
	//MutatingWebhookServicePath is the path for mutation webhook
	MutatingWebhookServicePath = "/mutate"
	//ValidatingWebhookServicePath is the path for validation webhook
	ValidatingWebhookServicePath = "/validate"
	//PolicyValidatingWebhookServicePath is the path for policy validation webhook(used to validate policy resource)
	PolicyValidatingWebhookServicePath = "/policyvalidate"
	//PolicyMutatingWebhookServicePath is the path for policy mutation webhook(used to default)
	PolicyMutatingWebhookServicePath = "/policymutate"
	//VerifyMutatingWebhookServicePath is the path for verify webhook(used to veryfing if admission control is enabled and active)
	VerifyMutatingWebhookServicePath = "/verifymutate"
	// LivenessServicePath is the path for check liveness health
	LivenessServicePath = "/health/liveness"
	// ReadinessServicePath is the path for check readness health
	ReadinessServicePath = "/health/readiness"
)

var (
	// KyvernoNamespace is the Kyverno namespace
	KyvernoNamespace = osutils.GetEnvWithFallback("KYVERNO_NAMESPACE", "kyverno")
	// KyvernoDeploymentName is the Kyverno deployment name
	KyvernoDeploymentName = osutils.GetEnvWithFallback("KYVERNO_DEPLOYMENT", "kyverno")
	// KyvernoServiceName is the Kyverno service name
	KyvernoServiceName = osutils.GetEnvWithFallback("KYVERNO_SVC", "kyverno-svc")
	// KyvernoPodName is the Kyverno pod name
	KyvernoPodName = osutils.GetEnvWithFallback("KYVERNO_POD_NAME", "kyverno")
	// KyvernoConfigMapName is the Kyverno configmap name
	KyvernoConfigMapName = osutils.GetEnvWithFallback("INIT_CONFIG", "kyverno")
	// defaultExcludeGroupRole ...
	defaultExcludeGroupRole []string = []string{"system:serviceaccounts:kube-system", "system:nodes", "system:kube-scheduler"}
)

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
	// RestrictDevelopmentUsername return exclude development username
	RestrictDevelopmentUsername() []string
	// FilterNamespaces filters exclude namespace
	FilterNamespaces(namespaces []string) []string
	// GetWebhooks returns the webhook configs
	GetWebhooks() []WebhookConfig
	// Load loads configuration from a configmap
	Load(cm *v1.ConfigMap)
}

// configuration stores the configuration
type configuration struct {
	mux                         sync.RWMutex
	filters                     []filter
	excludeGroupRole            []string
	excludeUsername             []string
	restrictDevelopmentUsername []string
	webhooks                    []WebhookConfig
	generateSuccessEvents       bool
	reconcilePolicyReport       chan<- bool
	updateWebhookConfigurations chan<- bool
}

// NewConfiguration ...
func NewConfiguration(client kubernetes.Interface, reconcilePolicyReport, updateWebhookConfigurations chan<- bool) (Configuration, error) {
	cd := &configuration{
		reconcilePolicyReport:       reconcilePolicyReport,
		updateWebhookConfigurations: updateWebhookConfigurations,
		restrictDevelopmentUsername: []string{"minikube-user", "kubernetes-admin"},
		excludeGroupRole:            defaultExcludeGroupRole,
	}
	if cm, err := client.CoreV1().ConfigMaps(KyvernoNamespace).Get(context.TODO(), KyvernoConfigMapName, metav1.GetOptions{}); err != nil {
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

func (cd *configuration) RestrictDevelopmentUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.restrictDevelopmentUsername
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

func (cd *configuration) Load(cm *v1.ConfigMap) {
	reconcilePolicyReport, updateWebhook := true, true
	if cm != nil {
		logger.Info("load config", "name", cm.Name, "namespace", cm.Namespace)
		reconcilePolicyReport, updateWebhook = cd.load(cm)
	} else {
		logger.Info("unload config")
		cd.unload()
	}
	if reconcilePolicyReport {
		logger.Info("resource filters changed, sending reconcile signal to the policy controller")
		cd.reconcilePolicyReport <- true
	}
	if updateWebhook {
		logger.Info("webhook configurations changed, updating webhook configurations")
		cd.updateWebhookConfigurations <- true
	}
}

func (cd *configuration) load(cm *v1.ConfigMap) (reconcilePolicyReport, updateWebhook bool) {
	logger := logger.WithValues("name", cm.Name, "namespace", cm.Namespace)
	if cm.Data == nil {
		logger.V(4).Info("configuration: No data defined in ConfigMap")
		return
	}
	cd.mux.Lock()
	defer cd.mux.Unlock()
	filters, ok := cm.Data["resourceFilters"]
	if !ok {
		logger.V(4).Info("configuration: No resourceFilters defined in ConfigMap")
	} else {
		newFilters := parseKinds(filters)
		if reflect.DeepEqual(newFilters, cd.filters) {
			logger.V(4).Info("resourceFilters did not change")
		} else {
			logger.V(2).Info("Updated resource filters", "oldFilters", cd.filters, "newFilters", newFilters)
			cd.filters = newFilters
			reconcilePolicyReport = true
		}
	}
	excludeGroupRole, ok := cm.Data["excludeGroupRole"]
	if !ok {
		logger.V(4).Info("configuration: No excludeGroupRole defined in ConfigMap")
	}
	newExcludeGroupRoles := parseRbac(excludeGroupRole)
	newExcludeGroupRoles = append(newExcludeGroupRoles, defaultExcludeGroupRole...)
	if reflect.DeepEqual(newExcludeGroupRoles, cd.excludeGroupRole) {
		logger.V(4).Info("excludeGroupRole did not change")
	} else {
		logger.V(2).Info("Updated resource excludeGroupRoles", "oldExcludeGroupRole", cd.excludeGroupRole, "newExcludeGroupRole", newExcludeGroupRoles)
		cd.excludeGroupRole = newExcludeGroupRoles
		reconcilePolicyReport = true
	}
	excludeUsername, ok := cm.Data["excludeUsername"]
	if !ok {
		logger.V(4).Info("configuration: No excludeUsername defined in ConfigMap")
	} else {
		excludeUsernames := parseRbac(excludeUsername)
		if reflect.DeepEqual(excludeUsernames, cd.excludeUsername) {
			logger.V(4).Info("excludeGroupRole did not change")
		} else {
			logger.V(2).Info("Updated resource excludeUsernames", "oldExcludeUsername", cd.excludeUsername, "newExcludeUsername", excludeUsernames)
			cd.excludeUsername = excludeUsernames
			reconcilePolicyReport = true
		}
	}
	webhooks, ok := cm.Data["webhooks"]
	if !ok {
		if len(cd.webhooks) > 0 {
			cd.webhooks = nil
			updateWebhook = true
			logger.V(4).Info("configuration: Setting namespaceSelector to empty in the webhook configurations")
		} else {
			logger.V(4).Info("configuration: No webhook configurations defined in ConfigMap")
		}
	} else {
		cfgs, err := parseWebhooks(webhooks)
		if err != nil {
			logger.Error(err, "unable to parse webhooks configurations")
			return
		}

		if reflect.DeepEqual(cfgs, cd.webhooks) {
			logger.V(4).Info("webhooks did not change")
		} else {
			logger.Info("Updated webhooks configurations", "oldWebhooks", cd.webhooks, "newWebhookd", cfgs)
			cd.webhooks = cfgs
			updateWebhook = true
		}
	}
	generateSuccessEvents, ok := cm.Data["generateSuccessEvents"]
	if !ok {
		logger.V(4).Info("configuration: No generateSuccessEvents defined in ConfigMap")
	} else {
		generateSuccessEvents, err := strconv.ParseBool(generateSuccessEvents)
		if err != nil {
			logger.V(4).Info("configuration: generateSuccessEvents must be either true/false")
		} else if generateSuccessEvents == cd.generateSuccessEvents {
			logger.V(4).Info("generateSuccessEvents did not change")
		} else {
			logger.V(2).Info("Updated generateSuccessEvents", "oldGenerateSuccessEvents", cd.generateSuccessEvents, "newGenerateSuccessEvents", generateSuccessEvents)
			cd.generateSuccessEvents = generateSuccessEvents
			reconcilePolicyReport = true
		}
	}
	return
}

func (cd *configuration) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []filter{}
	cd.excludeGroupRole = []string{}
	cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	cd.excludeUsername = []string{}
	cd.generateSuccessEvents = false
}
