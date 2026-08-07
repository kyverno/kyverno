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

// keychainSource is implemented by registry clients that expose the
// authn.Keychain used to resolve credentials. registryclient.Client (and
// therefore the global registry client the admission controller builds at
// startup) satisfies this.
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

	// preserve the global admission-controller keychain (IRSA, --registryCredentialHelpers,
	// Kyverno deployment pull secrets) so that building a per-request client below doesn't
	// silently drop it in favor of the policy/resource provided credentials. The global
	// keychain is given the first chance to authenticate; policy/resource credentials are
	// only used as a fallback.
	opts := []registryclient.Option{registryclient.WithSecretLister(f.secretsLister, resourceNamespace)}
	if global, ok := f.globalClient.(keychainSource); ok && global.Keychain() != nil {
		opts = append(opts, registryclient.WithKeychain(global.Keychain()))
	}

	// the policy contains extra credentials apart from whats passed in imagePullSecrets
	if creds != nil {
		if len(creds.Providers) > 0 {
			providers := make([]string, len(creds.Providers))
			for i, p := range creds.Providers {
				providers[i] = string(p)
			}
			opts = append(opts, registryclient.WithCredentialHelpers(providers...))
		}

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

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	if len(imagePullSecrets) > 0 {
		opts = append(opts, registryclient.WithImagePullSecrets(prefixSecretNamespaces(imagePullSecrets, resourceNamespace)...))
	}
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
