package internal

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/sdk/extensions/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func setupRegistryClient(ctx context.Context, logger logr.Logger, client kubernetes.Interface) (registryclient.Client, corev1listers.SecretLister) {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.V(2).Info("setup registry client...")
	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
	secretLister := factory.Core().V1().Secrets().Lister()
	// start informers and wait for cache sync
	if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
		checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
	}

	registryClient := registryclient.SetupGlobalRegistryClient(secretLister, config.KyvernoNamespace(),
		imagePullSecrets,
		registryCredentialHelpers, allowInsecureRegistry)

	return registryClient, secretLister
}
