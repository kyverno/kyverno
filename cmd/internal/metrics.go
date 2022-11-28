package internal

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/client-go/kubernetes"
)

func GetMetricsConfiguration(logger logr.Logger, client kubernetes.Interface) config.MetricsConfiguration {
	logger = logger.WithName("metrics")
	logger.Info("load metrics configuration...")
	metricsConfiguration, err := config.NewMetricsConfiguration(client)
	checkError(logger, err, "failed to load metrics configuration")
	return metricsConfiguration
}
