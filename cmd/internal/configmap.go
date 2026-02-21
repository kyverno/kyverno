package internal

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	genericconfigmapcontroller "github.com/kyverno/kyverno/pkg/controllers/generic/configmap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

func startConfigController(ctx context.Context, logger logr.Logger, client kubernetes.Interface, skipResourceFilters bool) config.Configuration {
	logger = logger.WithName("config-controller")
	logger.V(2).Info("create config-controller...")
	defer logger.V(2).Info("done creating config-controller")
	configuration := config.NewDefaultConfiguration(skipResourceFilters)
	configurationController := genericconfigmapcontroller.NewController(
		"config-controller",
		client,
		resyncPeriod,
		config.KyvernoNamespace(),
		config.KyvernoConfigMapName(),
		func(ctx context.Context, cm *corev1.ConfigMap) error {
			configuration.Load(cm)
			return nil
		},
	)
	checkError(logger, configurationController.WarmUp(ctx), "failed to init config controller")
	go configurationController.Run(ctx, 1)
	return configuration
}

func startMetricsConfigController(ctx context.Context, logger logr.Logger, client kubernetes.Interface) config.MetricsConfiguration {
	logger = logger.WithName("metrics-config-controller")
	logger.V(2).Info("create metrics-config-controller...")
	defer logger.V(2).Info("done creating metrics-config-controller")
	configuration := config.NewDefaultMetricsConfiguration()
	configurationController := genericconfigmapcontroller.NewController(
		"metrics-config-controller",
		client,
		resyncPeriod,
		config.KyvernoNamespace(),
		config.KyvernoMetricsConfigMapName(),
		func(ctx context.Context, cm *corev1.ConfigMap) error {
			configuration.Load(cm)
			return nil
		},
	)
	checkError(logger, configurationController.WarmUp(ctx), "failed to init metrics config controller")
	go configurationController.Run(ctx, 1)
	return configuration
}
