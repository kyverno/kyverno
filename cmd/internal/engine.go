package internal

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func NewEngine(
	ctx context.Context,
	logger logr.Logger,
	configuration config.Configuration,
	metricsConfiguration config.MetricsConfiguration,
	jp jmespath.Interface,
	client dclient.Interface,
	rclient registryclient.Client,
	ivCache imageverifycache.Client,
	dclient dynamic.Interface,
	kubeClient kubernetes.Interface,
	kyvernoClient versioned.Interface,
	secretLister corev1listers.SecretNamespaceLister,
	apiCallConfig apicall.APICallConfiguration,
) engineapi.Engine {
	configMapResolver := NewConfigMapResolver(ctx, logger, kubeClient, 15*time.Minute)
	exceptionsSelector := NewExceptionSelector(ctx, logger, kyvernoClient, 15*time.Minute)
	resourceCacheClient := NewResourceCacheLoader(ctx, logger, jp, kyvernoClient, dclient, apiCallConfig, resyncPeriod)
	logger = logger.WithName("engine")
	logger.Info("setup engine...")
	return engine.NewEngine(
		configuration,
		metricsConfiguration,
		jp,
		adapters.Client(client),
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), secretLister),
		ivCache,
		factories.DefaultContextLoaderFactory(configMapResolver, resourceCacheClient, factories.WithAPICallConfig(apiCallConfig)),
		exceptionsSelector,
		imageSignatureRepository,
	)
}

func NewExceptionSelector(
	ctx context.Context,
	logger logr.Logger,
	kyvernoClient versioned.Interface,
	resyncPeriod time.Duration,
) engineapi.PolicyExceptionSelector {
	logger = logger.WithName("exception-selector").WithValues("enablePolicyException", enablePolicyException, "exceptionNamespace", exceptionNamespace)
	logger.Info("setup exception selector...")
	var exceptionsLister engineapi.PolicyExceptionSelector
	if enablePolicyException {
		factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
		lister := factory.Kyverno().V2beta1().PolicyExceptions().Lister()
		if exceptionNamespace != "" {
			exceptionsLister = lister.PolicyExceptions(exceptionNamespace)
		} else {
			exceptionsLister = lister
		}
		// start informers and wait for cache sync
		if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
			checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		}
	}
	return exceptionsLister
}

func NewResourceCacheLoader(
	ctx context.Context,
	logger logr.Logger,
	jp jmespath.Interface,
	kyvernoClient versioned.Interface,
	dclient dynamic.Interface,
	apiCallConfig apicall.APICallConfiguration,
	resyncPeriod time.Duration,
) resourcecache.Interface {
	logger = logger.WithName("resourcecache-loader").WithValues("enableResourceCache", enableResourceCache)
	logger.Info("setup resource cache loader...")
	if !enableResourceCache {
		logger.V(4).Info("resource caching is disabled")
		return nil
	}
	var informer v2alpha1.CachedContextEntryInformer
	factory := kyvernoinformer.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	informer = factory.Kyverno().V2alpha1().CachedContextEntries()
	// start informers and wait for cache sync
	if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
		checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	}

	rc, err := resourcecache.New(logger, dclient, informer, jp, apiCallConfig)
	if err != nil {
		logger.Error(err, "failed to create resource cache client")
	}
	return rc
}

func NewConfigMapResolver(
	ctx context.Context,
	logger logr.Logger,
	kubeClient kubernetes.Interface,
	resyncPeriod time.Duration,
) engineapi.ConfigmapResolver {
	logger = logger.WithName("configmap-resolver").WithValues("enableConfigMapCaching", enableConfigMapCaching)
	logger.Info("setup config map resolver...")
	clientBasedResolver, err := resolvers.NewClientBasedResolver(kubeClient)
	checkError(logger, err, "failed to create client based resolver")
	if !enableConfigMapCaching {
		return clientBasedResolver
	}
	factory, err := resolvers.GetCacheInformerFactory(kubeClient, resyncPeriod)
	checkError(logger, err, "failed to create cache informer factory")
	informerBasedResolver, err := resolvers.NewInformerBasedResolver(factory.Core().V1().ConfigMaps().Lister())
	checkError(logger, err, "failed to create informer based resolver")
	configMapResolver, err := engineapi.NewNamespacedResourceResolver(informerBasedResolver, clientBasedResolver)
	checkError(logger, err, "failed to create config map resolver")
	// start informers and wait for cache sync
	if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
		checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	}
	return configMapResolver
}
