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

func DefaultRegistryClientFactory(globalClient engineapi.RegistryClient, secretsLister corev1listers.SecretLister, opts ...RegistryClientFactoryOption) engineapi.RegistryClientFactory {
	factory := &registryClientFactory{
		globalClient:  globalClient,
		secretsLister: secretsLister,
	}
	for _, option := range opts {
		if option != nil {
			option(factory)
		}
	}
	return factory
}

// RegistryClientFactoryOption configures clients created by the factory.
type RegistryClientFactoryOption func(*registryClientFactory)

// WithPrivateRegistryAllowlist configures exact hosts, IP addresses, and CIDRs
// permitted to reach private addresses in enforce mode.
func WithPrivateRegistryAllowlist(hosts string) RegistryClientFactoryOption {
	return func(factory *registryClientFactory) {
		if hosts == "" {
			factory.privateRegistryAllowlist = nil
		} else {
			factory.privateRegistryAllowlist = strings.Split(hosts, ",")
		}
	}
}

// WithPrivateRegistryEgressMode configures audit or enforce mode for private
// addresses on clients created for policy credentials or image pull secrets.
func WithPrivateRegistryEgressMode(mode string) RegistryClientFactoryOption {
	return func(factory *registryClientFactory) {
		factory.privateRegistryEgressMode = mode
	}
}

type registryClientFactory struct {
	globalClient              engineapi.RegistryClient
	secretsLister             corev1listers.SecretLister
	privateRegistryAllowlist  []string
	privateRegistryEgressMode string
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
		// turn the array of providers into a slice of credential helper names
		providers := make([]string, len(creds.Providers))
		for i, p := range creds.Providers {
			providers[i] = string(p)
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

		client := registryclient.New(
			registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
			registryclient.WithImagePullSecrets(secrets...),
			registryclient.WithCredentialHelpers(providers...),
			registryclient.WithAllowInsecureRegistry(creds.AllowInsecureRegistry),
			registryclient.WithPrivateRegistryAllowlist(f.privateRegistryAllowlist...),
			registryclient.WithEgressMode(f.privateRegistryEgressMode),
		)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	secrets := prefixSecretNamespaces(imagePullSecrets, resourceNamespace)
	client := registryclient.New(
		registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
		registryclient.WithImagePullSecrets(secrets...),
		registryclient.WithPrivateRegistryAllowlist(f.privateRegistryAllowlist...),
		registryclient.WithEgressMode(f.privateRegistryEgressMode),
	)
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
