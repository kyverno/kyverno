package config

import (
	"context"
	"os"
	"time"

	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// metricsConfigEnvVar is the name of an environment variable containing the name of the configmap
// that stores the information associated with Kyverno's metrics exposure
const metricsConfigEnvVar string = "METRICS_CONFIG"

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
}

// metricsConfig stores the config for metrics
type metricsConfig struct {
	namespaces             namespacesConfig
	metricsRefreshInterval time.Duration
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

// NewMetricsConfiguration ...
func NewMetricsConfiguration(client kubernetes.Interface) (MetricsConfiguration, error) {
	configuration := NewDefaultMetricsConfiguration()
	cmName := os.Getenv(metricsConfigEnvVar)
	if cmName != "" {
		if cm, err := client.CoreV1().ConfigMaps(kyvernoNamespace).Get(context.TODO(), cmName, metav1.GetOptions{}); err != nil {
			if !errors.IsNotFound(err) {
				return nil, err
			}
		} else {
			configuration.load(cm)
		}
	}
	return configuration, nil
}

// GetExcludeNamespaces returns the namespaces to ignore for metrics exposure
func (mcd *metricsConfig) GetExcludeNamespaces() []string {
	return mcd.namespaces.ExcludeNamespaces
}

// GetIncludeNamespaces returns the namespaces to specifically consider for metrics exposure
func (mcd *metricsConfig) GetIncludeNamespaces() []string {
	return mcd.namespaces.IncludeNamespaces
}

// GetMetricsRefreshInterval returns the refresh interval for the metrics
func (mcd *metricsConfig) GetMetricsRefreshInterval() time.Duration {
	return mcd.metricsRefreshInterval
}

// CheckNamespace returns `true` if the namespace has to be considered
func (mcd *metricsConfig) CheckNamespace(namespace string) bool {
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

func (cd *metricsConfig) load(cm *corev1.ConfigMap) {
	logger := logger.WithValues("name", cm.Name, "namespace", cm.Namespace)
	if cm.Data == nil {
		return
	}
	// reset
	cd.metricsRefreshInterval = 0
	cd.namespaces = namespacesConfig{
		IncludeNamespaces: []string{},
		ExcludeNamespaces: []string{},
	}
	// load metricsRefreshInterval
	metricsRefreshInterval, found := cm.Data["metricsRefreshInterval"]
	if found {
		metricsRefreshInterval, err := time.ParseDuration(metricsRefreshInterval)
		if err != nil {
			logger.Error(err, "failed to parse metricsRefreshInterval")
		} else {
			cd.metricsRefreshInterval = metricsRefreshInterval
		}
	}
	// load namespaces
	namespaces, ok := cm.Data["namespaces"]
	if ok {
		namespaces, err := parseIncludeExcludeNamespacesFromNamespacesConfig(namespaces)
		if err != nil {
			logger.Error(err, "failed to parse namespaces")
		} else {
			cd.namespaces = namespaces
		}
	}
}
