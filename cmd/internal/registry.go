package internal

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

func getRegistryClientLoader(ctx context.Context, logger logr.Logger, client kubernetes.Interface) engineapi.RegistryClientLoader {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	secretLister := factory.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
	// start informers and wait for cache sync
	if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
		checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	}
	registryClientLoaderFactory := engineapi.DefaultRegistryClientLoaderFactory(ctx, secretLister)
	return registryClientLoaderFactory(imagePullSecrets, allowInsecureRegistry, registryCredentialHelpers)
}
