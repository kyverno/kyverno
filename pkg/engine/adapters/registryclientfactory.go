package adapters

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"

	"k8s.io/client-go/kubernetes"
)

type registryClientFactory struct {
	globalClient engineapi.RegistryClient
}

func (f *registryClientFactory) GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials) (engineapi.RegistryClient, error) {
	if creds != nil {
		registryOptions := []registryclient.Option{
			registryclient.WithTracing(),
		}
		if creds.AllowInsecureRegistry {
			registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
		}
		if len(creds.Helpers) > 0 {
			var helpers []string
			for _, helper := range creds.Helpers {
				helpers = append(helpers, string(helper))
			}
			registryOptions = append(registryOptions, registryclient.WithCredentialHelpers(helpers...))
		}
		// secrets := strings.Split(imagePullSecrets, ",")
		// if imagePullSecrets != "" && len(secrets) > 0 {
		// 	factory := kubeinformers.NewSharedInformerFactoryWithOptions(client, resyncPeriod, kubeinformers.WithNamespace(config.KyvernoNamespace()))
		// 	secretLister := factory.Core().V1().Secrets().Lister().Secrets(config.KyvernoNamespace())
		// 	// start informers and wait for cache sync
		// 	if !StartInformersAndWaitForCacheSync(ctx, logger, factory) {
		// 		checkError(logger, errors.New("failed to wait for cache sync"), "failed to wait for cache sync")
		// 	}
		// 	registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(ctx, secretLister, secrets...))
		// }

		client, err := registryclient.New(registryOptions...)
		if err != nil {
			return nil, err
		}
		return RegistryClient(client), nil
	}
	return f.globalClient, nil
}

func DefaultRegistryClientFactory(globalClient engineapi.RegistryClient, kubeClient kubernetes.Interface) engineapi.RegistryClientFactory {
	return &registryClientFactory{
		globalClient: globalClient,
	}
}
