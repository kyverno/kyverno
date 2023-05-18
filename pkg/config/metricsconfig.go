package config

import (
	"sync"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
)

// MetricsConfig stores the config for metrics
type MetricsConfiguration interface {
	// GetExcludeNamespaces returns the namespaces to ignore for metrics exposure
	GetExcludeNamespaces() []string
	// GetIncludeNamespaces returns the namespaces to specifically consider for metrics exposure
	GetIncludeNamespaces() []string
	// GetMetricsRefreshInterval returns the refresh interval for the metrics
	GetMetricsRefreshInterval() time.Duration
	// CheckNamespace returns `true` if the namespace has to be considered
	CheckNamespace(string) bool
	// Load loads configuration from a configmap
	Load(*corev1.ConfigMap)
	// OnChanged adds a callback to be invoked when the configuration is reloaded
	OnChanged(func())
}

// metricsConfig stores the config for metrics
type metricsConfig struct {
	namespaces             namespacesConfig
	metricsRefreshInterval time.Duration
	mux                    sync.RWMutex
	callbacks              []func()
}

// NewDefaultMetricsConfiguration ...
func NewDefaultMetricsConfiguration() *metricsConfig {
	return &metricsConfig{
		metricsRefreshInterval: 0,
		namespaces: namespacesConfig{
			IncludeNamespaces: []string{},
			ExcludeNamespaces: []string{},
		},
	}
}

func (cd *metricsConfig) OnChanged(callback func()) {
	cd.mux.Lock()
	defer cd.mux.Unlock()
	cd.callbacks = append(cd.callbacks, callback)
}

// GetExcludeNamespaces returns the namespaces to ignore for metrics exposure
func (mcd *metricsConfig) GetExcludeNamespaces() []string {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()
	return mcd.namespaces.ExcludeNamespaces
}

// GetIncludeNamespaces returns the namespaces to specifically consider for metrics exposure
func (mcd *metricsConfig) GetIncludeNamespaces() []string {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()
	return mcd.namespaces.IncludeNamespaces
}

// GetMetricsRefreshInterval returns the refresh interval for the metrics
func (mcd *metricsConfig) GetMetricsRefreshInterval() time.Duration {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()
	return mcd.metricsRefreshInterval
}

// CheckNamespace returns `true` if the namespace has to be considered
func (mcd *metricsConfig) CheckNamespace(namespace string) bool {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()
	// TODO(eddycharly): check we actually need `"-"`
	if namespace == "" || namespace == "-" {
		return true
	}
	if slices.Contains(mcd.namespaces.ExcludeNamespaces, namespace) {
		return false
	}
	if len(mcd.namespaces.IncludeNamespaces) == 0 {
		return true
	}
	return slices.Contains(mcd.namespaces.IncludeNamespaces, namespace)
}

func (mcd *metricsConfig) Load(cm *corev1.ConfigMap) {
	if cm != nil {
		mcd.load(cm)
	} else {
		mcd.unload()
	}
}

func (cd *metricsConfig) load(cm *corev1.ConfigMap) {
	logger := logger.WithValues("name", cm.Name, "namespace", cm.Namespace)
	cd.mux.Lock()
	defer cd.mux.Unlock()
	defer cd.notify()
	data := cm.Data
	if data == nil {
		data = map[string]string{}
	}
	// reset
	cd.metricsRefreshInterval = 0
	cd.namespaces = namespacesConfig{
		IncludeNamespaces: []string{},
		ExcludeNamespaces: []string{},
	}
	// load metricsRefreshInterval
	metricsRefreshInterval, ok := data["metricsRefreshInterval"]
	if !ok {
		logger.Info("metricsRefreshInterval not set")
	} else {
		logger := logger.WithValues("metricsRefreshInterval", metricsRefreshInterval)
		metricsRefreshInterval, err := time.ParseDuration(metricsRefreshInterval)
		if err != nil {
			logger.Error(err, "failed to parse metricsRefreshInterval")
		} else {
			cd.metricsRefreshInterval = metricsRefreshInterval
			logger.Info("metricsRefreshInterval configured")
		}
	}
	// load namespaces
	namespaces, ok := data["namespaces"]
	if !ok {
		logger.Info("namespaces not set")
	} else {
		logger := logger.WithValues("namespaces", namespaces)
		namespaces, err := parseIncludeExcludeNamespacesFromNamespacesConfig(namespaces)
		if err != nil {
			logger.Error(err, "failed to parse namespaces")
		} else {
			cd.namespaces = namespaces
			logger.Info("namespaces configured")
		}
	}
}

func (mcd *metricsConfig) unload() {
	mcd.mux.Lock()
	defer mcd.mux.Unlock()
	defer mcd.notify()
	mcd.metricsRefreshInterval = 0
	mcd.namespaces = namespacesConfig{
		IncludeNamespaces: []string{},
		ExcludeNamespaces: []string{},
	}
}

func (mcd *metricsConfig) notify() {
	for _, callback := range mcd.callbacks {
		callback()
	}
}
