package factories

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// the registry client used in this factory is a wrapper over the normal
// registry client provided by the SDK package and hence the Keychain()
// method is not available. We will need to type assert the client into this
// interface so we can fetch its keychain
type keychainSource interface {
	Keychain() authn.Keychain
}

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

	// extract any credential providers built with the global client to still be able to use
	// other authentication methods against image registries made available to kyverno (AWS IRSA, GCP
	// service accounts, etc..)
	opts := []registryclient.Option{registryclient.WithSecretLister(f.secretsLister, resourceNamespace)}
	if global, ok := f.globalClient.(keychainSource); ok && global.Keychain() != nil {
		opts = append(opts, registryclient.WithKeychain(global.Keychain()))
	}

	// the policy contains extra credentials apart from whats passed in imagePullSecrets
	if creds != nil {
		if len(creds.Providers) > 0 {
			// turn the array of providers into a slice of credential helper names
			providers := make([]string, len(creds.Providers))
			for i, p := range creds.Providers {
				providers[i] = string(p)
			}
			opts = append(opts, registryclient.WithCredentialHelpers(providers...))
		}

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
		if len(secrets) > 0 {
			opts = append(opts, registryclient.WithImagePullSecrets(secrets...))
		}

		if creds.AllowInsecureRegistry {
			opts = append(opts, registryclient.WithAllowInsecureRegistry(true))
		}

		client := registryclient.New(opts...)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with the imagePullSecrets layered under the global keychain
	opts = append(opts, registryclient.WithImagePullSecrets(prefixSecretNamespaces(imagePullSecrets, resourceNamespace)...))
	client := registryclient.New(opts...)
	return adapters.RegistryClient(client), nil
}

// prefixSecretNamespaces prefixes each secret ref with defaultNamespace unless it already
// uses namespace/name notation.
func prefixSecretNamespaces(secrets []string, defaultNamespace string) []string {
	prefixed := make([]string, len(secrets))
	for i, s := range secrets {
		s = strings.TrimPrefix(s, "/")
		if strings.Contains(s, "/") {
			prefixed[i] = s
		} else {
			prefixed[i] = defaultNamespace + "/" + s
		}
	}
	return prefixed
}
