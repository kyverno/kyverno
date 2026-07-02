package factories

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/sdk/extensions/registryclient"
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
	if resourceNamespace == "" {
		resourceNamespace = config.KyvernoNamespace()
	}

	if creds == nil && len(imagePullSecrets) == 0 {
		return f.globalClient, nil
	}

	// the policy contains extra credentials apart from whats passed in imagePullSecrets
	if creds != nil {
		// turn the array of providers to a single comma separated string
		strs := make([]string, len(creds.Providers))
		for i, p := range creds.Providers {
			strs[i] = string(p)
		}
		providers := strings.Join(strs, ",")

		// create an array of secret names where we will accumulate whats in creds and imagePullSecrets
		// creds.Secrets default to the Kyverno namespace, imagePullSecrets default to the resource namespace,
		// so each list must be prefixed independently before merging.
		secrets := make([]string, 0)
		if f.secretsLister != nil && len(creds.Secrets) > 0 {
			secrets = append(secrets, prefixSecretNamespaces(creds.Secrets, config.KyvernoNamespace())...)
		}
		if len(imagePullSecrets) > 0 {
			secrets = append(secrets, prefixSecretNamespaces(imagePullSecrets, resourceNamespace)...)
		}

		secretsJoined := strings.Join(secrets, ",")
		client := registryclient.New(f.secretsLister, resourceNamespace, secretsJoined, providers, creds.AllowInsecureRegistry)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	secretsJoined := strings.Join(prefixSecretNamespaces(imagePullSecrets, resourceNamespace), ",")
	client := registryclient.New(f.secretsLister, resourceNamespace, secretsJoined, "", false)
	return adapters.RegistryClient(client), nil
}

// prefixSecretNamespaces prefixes each secret ref with defaultNamespace unless it already
// uses namespace/name notation.
func prefixSecretNamespaces(secrets []string, defaultNamespace string) []string {
	prefixed := make([]string, len(secrets))
	for i, s := range secrets {
		if strings.Contains(s, "/") {
			prefixed[i] = s
		} else {
			prefixed[i] = defaultNamespace + "/" + s
		}
	}
	return prefixed
}
