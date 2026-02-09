package factories

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

func DefaultRegistryClientFactory(globalClient engineapi.RegistryClient, secretsLister corev1listers.SecretLister) engineapi.RegistryClientFactory {
	return &registryClientFactory{
		globalClient:  globalClient,
		secretsLister: secretsLister,
	}
}

type registryClientFactory struct {
	globalClient  engineapi.RegistryClient
	secretsLister corev1listers.SecretLister
}

func (f *registryClientFactory) GetClient(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string) (engineapi.RegistryClient, error) {
	if creds != nil || len(imagePullSecrets) > 0 {
		registryOptions := []registryclient.Option{
			registryclient.WithTracing(),
		}
		if creds != nil && creds.AllowInsecureRegistry {
			registryOptions = append(registryOptions, registryclient.WithAllowInsecureRegistry())
		}
		if creds != nil && len(creds.Providers) > 0 {
			var providers []string
			for _, helper := range creds.Providers {
				providers = append(providers, string(helper))
			}
			registryOptions = append(registryOptions, registryclient.WithCredentialProviders(providers...))
		}

		secrets := make([]string, 0)
		if creds != nil && f.secretsLister != nil && len(creds.Secrets) > 0 {
			secrets = append(secrets, creds.Secrets...)
		}
		if len(imagePullSecrets) > 0 {
			fallbackNamespace := resourceNamespace
			if strings.TrimSpace(fallbackNamespace) == "" {
				fallbackNamespace = config.KyvernoNamespace()
			}
			for _, s := range imagePullSecrets {
				secretNamespace, secretName := registryclient.ParseSecretReference(s, fallbackNamespace)
				secrets = append(secrets, secretNamespace+"/"+secretName)
			}
		}
		if f.secretsLister != nil && len(secrets) > 0 {
			// Support namespace/name notation with Kyverno namespace as default
			registryOptions = append(registryOptions, registryclient.WithKeychainPullSecrets(f.secretsLister, config.KyvernoNamespace(), secrets...))
		}
		client, err := registryclient.New(registryOptions...)
		if err != nil {
			return nil, err
		}
		return adapters.RegistryClient(client), nil
	}
	return f.globalClient, nil
}
