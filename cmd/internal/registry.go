package internal

import (
	"context"
	"errors"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
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
	registryOptions := []registryclient.Option{
		registryclient.WithTracing(),
	}
	secrets := splitAndTrim(imagePullSecrets)
	if len(secrets) > 0 {
		registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(secretLister, config.KyvernoNamespace(), secrets...))
	}
	if allowInsecureRegistry {
		registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
	}
	providers := splitAndTrim(registryCredentialHelpers)
	if len(providers) > 0 {
		registryOptions = append(registryOptions, registryclient.WithCredentialProviders(providers...))
	}
	registryClient, err := registryclient.New(registryOptions...)
	checkError(logger, err, "failed to create registry client")
	return registryClient, secretLister
}

func imageLoaderOptions() []imagedataloader.Option {
	return imagedataloader.BuildRemoteOpts(
		splitAndTrim(imagePullSecrets),
		splitAndTrim(registryCredentialHelpers),
		allowInsecureRegistry,
	)
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}

	if len(out) == 0 {
		return nil
	}

	return out
}
