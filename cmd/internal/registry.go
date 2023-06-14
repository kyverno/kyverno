package internal

import (
	"context"
	"errors"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

func setupRegistryClient(ctx context.Context, logger logr.Logger, client kubernetes.Interface) registryclient.Client {
	logger = logger.WithName("registry-client").WithValues("secrets", imagePullSecrets, "insecure", allowInsecureRegistry)
	logger.Info("setup registry client...")
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := strings.Split(imagePullSecrets, ",")
	if imagePullSecrets != "" && len(secrets) > 0 {
		factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
		secretLister := factory.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
		// start informers and wait for cache sync
		if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
			checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		}
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(ctx, secretLister, secrets...))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	if len(registryCredentialHelpers) > 0 {
		registryOptions = append(registryOptions, registryclient.WithCredentialHelpers(strings.Split(registryCredentialHelpers, ",")...))
	}
	registryClient, err := registryclient.New(registryOptions...)
	checkError(logger, err, "failed to create registry client")
	return registryClient
}
