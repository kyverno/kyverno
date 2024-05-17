package factories

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func DefaultRegistryClientFactory(globalClient engineapi.RegistryClient, secretsLister corev1listers.SecretNamespaceLister) engineapi.RegistryClientFactory {
	return &registryClientFactory{
		globalClient:  globalClient,
		secretsLister: secretsLister,
	}
}

type registryClientFactory struct {
	globalClient  engineapi.RegistryClient
	secretsLister corev1listers.SecretNamespaceLister
}

func (f *registryClientFactory) GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials) (engineapi.RegistryClient, error) {
	if creds != nil {
		registryOptions := []registryclient.Option{
			registryclient.WithTracing(),
		}
		if creds.AllowInsecureRegistry {
			registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
		}
		if len(creds.Providers) > 0 {
			var providers []string
			for _, helper := range creds.Providers {
				providers = append(providers, string(helper))
			}
			registryOptions = append(registryOptions, registryclient.WithCredentialProviders(providers...))
		}
		if len(creds.Secrets) > 0 {
			registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(f.secretsLister, creds.Secrets...))
		}
		client, err := registryclient.New(registryOptions...)
		if err != nil {
			return nil, err
		}
		return adapters.RegistryClient(client), nil
	}
	return f.globalClient, nil
}
