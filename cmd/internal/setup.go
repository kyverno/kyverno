package internal

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	apiserverclient "github.com/kyverno/kyverno/pkg/clients/apiserver"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	dynamicclient "github.com/kyverno/kyverno/pkg/clients/dynamic"
	kubeclient "github.com/kyverno/kyverno/pkg/clients/kube"
	kyvernoclient "github.com/kyverno/kyverno/pkg/clients/kyverno"
	metadataclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	Logger                 logr.Logger
	Configuration          config.Configuration
	MetricsConfiguration   config.MetricsConfiguration
	MetricsManager         metrics.MetricsConfigManager
	Jp                     jmespath.Interface
	KubeClient             kubeclient.UpstreamInterface
	LeaderElectionClient   kubeclient.UpstreamInterface
	RegistryClient         registryclient.Client
	ImageVerifyCacheClient imageverifycache.Client
	RegistrySecretLister   corev1listers.SecretNamespaceLister
	KyvernoClient          kyvernoclient.UpstreamInterface
	DynamicClient          dynamicclient.UpstreamInterface
	ApiServerClient        apiserverclient.UpstreamInterface
	MetadataClient         metadataclient.UpstreamInterface
	KyvernoDynamicClient   dclient.Interface
}

func Setup(config Configuration, name string, skipResourceFilters bool) (context.Context, SetupResult, context.CancelFunc) {
	logger := setupLogger()
	showVersion(logger)
	printFlagSettings(logger)
	showWarnings(config, logger)
	check(logger)
	sdownMaxProcs := setupMaxProcs(logger)
	setupProfiling(logger)
	ctx, sdownSignals := setupSignals(logger)
	client := kubeclient.From(createKubernetesClient(logger), kubeclient.WithTracing())
	metricsConfiguration := startMetricsConfigController(ctx, logger, client)
	metricsManager, sdownMetrics := SetupMetrics(ctx, logger, metricsConfiguration, client)
	client = client.WithMetrics(metricsManager, metrics.KubeClient)
	configuration := startConfigController(ctx, logger, client, skipResourceFilters)
	sdownTracing := SetupTracing(logger, name, client)
	var registryClient registryclient.Client
	var registrySecretLister corev1listers.SecretNamespaceLister
	if config.UsesRegistryClient() {
		registryClient, registrySecretLister = setupRegistryClient(ctx, logger, client)
	}
	var imageVerifyCache imageverifycache.Client
	if config.UsesImageVerifyCache() {
		imageVerifyCache = setupImageVerifyCache(ctx, logger)
	}
	if config.UsesCosign() {
		setupSigstoreTUF(ctx, logger)
	}
	var leaderElectionClient kubeclient.UpstreamInterface
	if config.UsesLeaderElection() {
		leaderElectionClient = createKubernetesClient(logger, kubeclient.WithMetrics(metricsManager, metrics.KubeClient), kubeclient.WithTracing())
	}
	var kyvernoClient kyvernoclient.UpstreamInterface
	if config.UsesKyvernoClient() {
		kyvernoClient = createKyvernoClient(logger, kyvernoclient.WithMetrics(metricsManager, metrics.KyvernoClient), kyvernoclient.WithTracing())
	}
	var dynamicClient dynamicclient.UpstreamInterface
	if config.UsesDynamicClient() {
		dynamicClient = createDynamicClient(logger, dynamicclient.WithMetrics(metricsManager, metrics.DynamicClient), dynamicclient.WithTracing())
	}
	var apiServerClient apiserverclient.UpstreamInterface
	if config.UsesApiServerClient() {
		apiServerClient = createApiServerClient(logger, apiserverclient.WithMetrics(metricsManager, metrics.ApiServerClient), apiserverclient.WithTracing())
	}
	var dClient dclient.Interface
	if config.UsesKyvernoDynamicClient() {
		dClient = createKyvernoDynamicClient(logger, ctx, dynamicClient, client, 15*time.Minute)
	}
	var metadataClient metadataclient.UpstreamInterface
	if config.UsesMetadataClient() {
		metadataClient = createMetadataClient(logger, metadataclient.WithMetrics(metricsManager, metrics.MetadataClient), metadataclient.WithTracing())
	}
	return ctx,
		SetupResult{
			Logger:                 logger,
			Configuration:          configuration,
			MetricsConfiguration:   metricsConfiguration,
			MetricsManager:         metricsManager,
			Jp:                     jmespath.New(configuration),
			KubeClient:             client,
			LeaderElectionClient:   leaderElectionClient,
			RegistryClient:         registryClient,
			ImageVerifyCacheClient: imageVerifyCache,
			RegistrySecretLister:   registrySecretLister,
			KyvernoClient:          kyvernoClient,
			DynamicClient:          dynamicClient,
			ApiServerClient:        apiServerClient,
			MetadataClient:         metadataClient,
			KyvernoDynamicClient:   dClient,
		},
		shutdown(logger.WithName("shutdown"), sdownMaxProcs, sdownMetrics, sdownTracing, sdownSignals)
}
