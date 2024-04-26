package config

import (
	"maps"
	"slices"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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
	// GetBucketBoundaries returns the bucket boundaries for Histogram metrics
	GetBucketBoundaries() []float64
	// BuildMeterProviderViews returns OTL view removing attributes which were disabled in the config
	BuildMeterProviderViews() []sdkmetric.View
	// Load loads configuration from a configmap
	Load(*corev1.ConfigMap)
	// OnChanged adds a callback to be invoked when the configuration is reloaded
	OnChanged(func())
}

// metricsConfig stores the config for metrics
type metricsConfig struct {
	namespaces             namespacesConfig
	metricsRefreshInterval time.Duration
	bucketBoundaries       []float64
	metricsExposure        map[string]metricExposureConfig
	mux                    sync.RWMutex
	callbacks              []func()
}

// NewDefaultMetricsConfiguration ...
func NewDefaultMetricsConfiguration() *metricsConfig {
	config := metricsConfig{}
	config.reset()
	return &config
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

// GetBucketBoundaries returns the bucket boundaries for Histogram metrics
func (mcd *metricsConfig) GetBucketBoundaries() []float64 {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()
	return mcd.bucketBoundaries
}

func (mcd *metricsConfig) BuildMeterProviderViews() []sdkmetric.View {
	mcd.mux.RLock()
	defer mcd.mux.RUnlock()

	views := []sdkmetric.View{}

	if len(mcd.metricsExposure) > 0 {
		metricsExposure := maps.Clone(mcd.metricsExposure)
		views = append(views, func(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
			s := sdkmetric.Stream{Name: i.Name, Description: i.Description, Unit: i.Unit}

			config, exists := metricsExposure[i.Name]
			if !exists {
				return s, false
			}

			if config.Enabled != nil && !*config.Enabled {
				s.Aggregation = sdkmetric.AggregationDrop{}
				return s, true
			}

			if len(config.DisabledLabelDimensions) > 0 {
				s.AttributeFilter = func(kv attribute.KeyValue) bool {
					return !slices.Contains(config.DisabledLabelDimensions, string(kv.Key))
				}
			}

			if len(config.BucketBoundaries) > 0 {
				aggregation := sdkmetric.DefaultAggregationSelector(i.Kind)
				switch a := aggregation.(type) {
				case sdkmetric.AggregationExplicitBucketHistogram:
					a.Boundaries = config.BucketBoundaries
					a.NoMinMax = false
					s.Aggregation = a
				}
			}

			return s, true
		})
	}

	return views
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
	cd.reset()
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
	// load bucket boundaries
	bucketBoundariesString, ok := data["bucketBoundaries"]
	if !ok {
		logger.Info("bucketBoundaries not set")
	} else {
		logger := logger.WithValues("bucketBoundaries", bucketBoundariesString)
		bucketBoundaries, err := parseBucketBoundariesConfig(bucketBoundariesString)
		if err != nil {
			logger.Error(err, "failed to parse bucketBoundariesString")
		} else {
			cd.bucketBoundaries = bucketBoundaries
			logger.Info("bucketBoundaries configured")
		}
	}
	// load include resource details
	metricsExposureString, ok := data["metricsExposure"]
	if !ok {
		logger.Info("metricsExposure not set")
	} else {
		logger := logger.WithValues("metricsExposure", metricsExposureString)
		metricsExposure, err := parseMetricExposureConfig(metricsExposureString, cd.bucketBoundaries)
		if err != nil {
			logger.Error(err, "failed to parse metricsExposure")
		} else {
			cd.metricsExposure = metricsExposure
			logger.Info("metricsExposure configured")
		}
	}
}

func (mcd *metricsConfig) unload() {
	mcd.mux.Lock()
	defer mcd.mux.Unlock()
	defer mcd.notify()
	mcd.reset()
}

func (mcd *metricsConfig) reset() {
	mcd.metricsRefreshInterval = 0
	mcd.namespaces = namespacesConfig{
		IncludeNamespaces: []string{},
		ExcludeNamespaces: []string{},
	}
	mcd.bucketBoundaries = []float64{
		0.005,
		0.01,
		0.025,
		0.05,
		0.1,
		0.25,
		0.5,
		1,
		2.5,
		5,
		10,
		15,
		20,
		25,
		30,
	}
	mcd.metricsExposure = map[string]metricExposureConfig{}
}

func (mcd *metricsConfig) notify() {
	for _, callback := range mcd.callbacks {
		callback()
	}
}
