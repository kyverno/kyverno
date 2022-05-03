package config

import (
	"reflect"
	"strconv"
	"sync"

	wildcard "github.com/kyverno/go-wildcard"
	v1 "k8s.io/api/core/v1"
)

var defaultExcludeGroupRole []string = []string{"system:serviceaccounts:kube-system", "system:nodes", "system:kube-scheduler"}

// Interface to be used by consumer to check filters
type Interface interface {
	ToFilter(kind, namespace, name string) bool
	GetExcludeGroupRole() []string
	GetExcludeUsername() []string
	GetGenerateSuccessEvents() bool
	RestrictDevelopmentUsername() []string
	FilterNamespaces(namespaces []string) []string
	GetWebhooks() []WebhookConfig
	Load(cm *v1.ConfigMap)
}

// configData stores the configuration
type configData struct {
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

// NewConfigData ...
func NewConfigData(filterK8sResources, excludeGroupRole, excludeUsername string, reconcilePolicyReport, updateWebhookConfigurations chan<- bool) Interface {
	cd := &configData{
		reconcilePolicyReport:       reconcilePolicyReport,
		updateWebhookConfigurations: updateWebhookConfigurations,
		restrictDevelopmentUsername: []string{"minikube-user", "kubernetes-admin"},
	}
	if filterK8sResources != "" {
		logger.Info("init configuration from commandline arguments for filterK8sResources")
		cd.initFilters(filterK8sResources)
	}
	if excludeGroupRole != "" {
		logger.Info("init configuration from commandline arguments for excludeGroupRole")
		cd.initRbac("excludeRoles", excludeGroupRole)
	} else {
		cd.initRbac("excludeRoles", "")
	}
	if excludeUsername != "" {
		logger.Info("init configuration from commandline arguments for excludeUsername")
		cd.initRbac("excludeUsername", excludeUsername)
	}
	return cd
}

// ToFilter checks if the given resource is set to be filtered in the configuration
func (cd *configData) ToFilter(kind, namespace, name string) bool {
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

// GetExcludeGroupRole return exclude roles
func (cd *configData) GetExcludeGroupRole() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeGroupRole
}

// RestrictDevelopmentUsername return exclude development username
func (cd *configData) RestrictDevelopmentUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.restrictDevelopmentUsername
}

// GetExcludeUsername return exclude username
func (cd *configData) GetExcludeUsername() []string {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.excludeUsername
}

// GetGenerateSuccessEvents return if should generate success events
func (cd *configData) GetGenerateSuccessEvents() bool {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.generateSuccessEvents
}

// FilterNamespaces filters exclude namespace
func (cd *configData) FilterNamespaces(namespaces []string) []string {
	var results []string
	for _, ns := range namespaces {
		if !cd.ToFilter("", ns, "") {
			results = append(results, ns)
		}
	}
	return results
}

// GetWebhooks returns the webhook configs
func (cd *configData) GetWebhooks() []WebhookConfig {
	cd.mux.RLock()
	defer cd.mux.RUnlock()
	return cd.webhooks
}

func (cd *configData) Load(cm *v1.ConfigMap) {
	reconcilePolicyReport, updateWebhook := true, true
	if cm != nil {
		reconcilePolicyReport, updateWebhook = cd.load(cm)
	} else {
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

func (cd *configData) load(cm *v1.ConfigMap) (reconcilePolicyReport, updateWebhook bool) {
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

func (cd *configData) unload() {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.filters = []filter{}
	cd.excludeGroupRole = []string{}
	cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	cd.excludeUsername = []string{}
	cd.generateSuccessEvents = false
}

func (cd *configData) initFilters(filters string) {
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()
	newFilters := parseKinds(filters)
	logger.V(2).Info("Init resource filters", "filters", newFilters)
	// update filters
	cd.filters = newFilters
}

func (cd *configData) initRbac(action, exclude string) {
	// parse and load the configuration
	cd.mux.Lock()
	defer cd.mux.Unlock()
	rbac := parseRbac(exclude)
	logger.V(2).Info("Init resource ", action, exclude)
	// update filters
	if action == "excludeRoles" {
		cd.excludeGroupRole = rbac
		cd.excludeGroupRole = append(cd.excludeGroupRole, defaultExcludeGroupRole...)
	} else {
		cd.excludeUsername = rbac
	}
}
