package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// read the conifgMap with name in env:METRICS_CONFIG
// this configmap stores the information associated with Kyverno's metrics exposure
const metricsCmName string = "METRICS_CONFIG"

// MetricsConfigData stores the metrics-related configuration
type MetricsConfigData struct {
	client        kubernetes.Interface
	cmName        string
	metricsConfig MetricsConfig
	log           logr.Logger
}

// MetricsConfig stores the config for metrics
type MetricsConfig struct {
	namespaces             namespacesConfig
	metricsRefreshInterval time.Duration
}

type namespacesConfig struct {
	IncludeNamespaces []string `json:"include,omitempty"`
	ExcludeNamespaces []string `json:"exclude,omitempty"`
}

// GetExcludeNamespaces returns the namespaces to ignore for metrics exposure
func (mcd *MetricsConfigData) GetExcludeNamespaces() []string {
	return mcd.metricsConfig.namespaces.ExcludeNamespaces
}

// GetIncludeNamespaces returns the namespaces to specifically consider for metrics exposure
func (mcd *MetricsConfigData) GetIncludeNamespaces() []string {
	return mcd.metricsConfig.namespaces.IncludeNamespaces
}

// GetMetricsRefreshInterval returns the refresh interval for the metrics
func (mcd *MetricsConfigData) GetMetricsRefreshInterval() time.Duration {
	return mcd.metricsConfig.metricsRefreshInterval
}

// GetMetricsConfigMapName returns the configmap name for the metric
func (mcd *MetricsConfigData) GetMetricsConfigMapName() string {
	return mcd.cmName
}

// NewMetricsConfigData ...
func NewMetricsConfigData(rclient kubernetes.Interface, log logr.Logger) (*MetricsConfigData, error) {
	// environment var is read at start only
	if metricsCmName == "" {
		log.Info("ConfigMap name not defined in env:METRICS_CONFIG: loading no default configuration")
	}

	mcd := MetricsConfigData{
		client: rclient,
		cmName: os.Getenv(metricsCmName),
		log:    log,
	}

	kyvernoNamespace := getKyvernoNameSpace()
	configMap, err := rclient.CoreV1().ConfigMaps(kyvernoNamespace).Get(context.TODO(), mcd.cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error occurred while fetching the metrics configmap at %s/%s: %w", kyvernoNamespace, mcd.cmName, err)
	}
	// parsing namespace-related config from the config map
	namespacesConfigStr, found := configMap.Data["namespaces"]
	if !found {
		mcd.metricsConfig.namespaces.IncludeNamespaces = []string{}
		mcd.metricsConfig.namespaces.ExcludeNamespaces = []string{}
	} else {
		mcd.metricsConfig.namespaces.IncludeNamespaces, mcd.metricsConfig.namespaces.ExcludeNamespaces, err = parseIncludeExcludeNamespacesFromNamespacesConfig(namespacesConfigStr)
		if err != nil {
			return nil, fmt.Errorf("error occurred while parsing the 'namespaces' field of metrics config map: %w", err)
		}
	}

	// parsing metricsRefreshInterval from the config map
	metricsRefreshInterval, found := configMap.Data["metricsRefreshInterval"]
	if found {
		mcd.metricsConfig.metricsRefreshInterval, err = time.ParseDuration(metricsRefreshInterval)
		if err != nil {
			return nil, fmt.Errorf("error occurred while parsing metricsRefreshInterval: %w", err)
		}
	} else {
		mcd.metricsConfig.metricsRefreshInterval = 0
	}

	return &mcd, nil
}

func parseIncludeExcludeNamespacesFromNamespacesConfig(jsonStr string) ([]string, []string, error) {
	var namespacesConfigObject *namespacesConfig
	err := json.Unmarshal([]byte(jsonStr), &namespacesConfigObject)
	if err != nil {
		return nil, nil, err
	}
	return namespacesConfigObject.IncludeNamespaces, namespacesConfigObject.ExcludeNamespaces, nil
}
