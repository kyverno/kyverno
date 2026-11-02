package internal

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"k8s.io/client-go/kubernetes"
)

func NewConfigMapResolver(
	ctx context.Context,
	logger logr.Logger,
	kubeClient kubernetes.Interface,
	resyncPeriod time.Duration,
) engineapi.ConfigmapResolver {
	logger = logger.WithName("configmap-resolver").WithValues("enableConfigMapCaching", enableConfigMapCaching)
	logger.V(2).Info("setup config map resolver...")
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
