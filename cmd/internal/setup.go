package internal

import (
	"context"

	"github.com/go-logr/logr"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"k8s.io/client-go/kubernetes"
)

func shutdown(logger logr.Logger, sdowns ...context.CancelFunc) context.CancelFunc {
	return func() {
		for i := range sdowns {
			if sdowns[i] != nil {
				logger.Info("shutting down...")
				defer sdowns[i]()
			}
		}
	}
}

type SetupResult struct {
	Logger               logr.Logger
	Configuration        config.Configuration
	MetricsConfiguration config.MetricsConfiguration
	MetricsManager       metrics.MetricsConfigManager
	Jp                   jmespath.Interface
	KubeClient           kubernetes.Interface
	LeaderElectionClient kubernetes.Interface
	RegistryClient       registryclient.Client
}

func Setup(config Configuration, name string, skipResourceFilters bool) (context.Context, SetupResult, context.CancelFunc) {
	logger := setupLogger()
	showVersion(logger)
	sdownMaxProcs := setupMaxProcs(logger)
	setupProfiling(logger)
	ctx, sdownSignals := setupSignals(logger)
	client := kubeclient.From(createKubernetesClient(logger), kubeclient.WithTracing())
	metricsConfiguration := startMetricsConfigController(ctx, logger, client)
	metricsManager, sdownMetrics := SetupMetrics(ctx, logger, metricsConfiguration, client)
	client = client.WithMetrics(metricsManager, metrics.KubeClient)
	configuration := startConfigController(ctx, logger, client, skipResourceFilters)
	sdownTracing := SetupTracing(logger, name, client)
	setupCosign(logger)
	var registryClient registryclient.Client
	if config.UsesRegistryClient() {
		registryClient = setupRegistryClient(ctx, logger, client)
	}
	var leaderElectionClient kubernetes.Interface
	if config.UsesLeaderElection() {
		leaderElectionClient = createKubernetesClient(logger, kubeclient.WithMetrics(metricsManager, metrics.KubeClient), kubeclient.WithTracing())
	}
	return ctx,
		SetupResult{
			Logger:               logger,
			Configuration:        configuration,
			MetricsConfiguration: metricsConfiguration,
			MetricsManager:       metricsManager,
			Jp:                   jmespath.New(configuration),
			KubeClient:           client,
			LeaderElectionClient: leaderElectionClient,
			RegistryClient:       registryClient,
		},
		shutdown(logger.WithName("shutdown"), sdownMaxProcs, sdownMetrics, sdownTracing, sdownSignals)
}
