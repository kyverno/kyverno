package factories

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1 "k8s.io/api/core/v1"
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
		)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	secrets := prefixSecretNamespaces(imagePullSecrets, resourceNamespace)
	client := registryclient.New(
		registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
		registryclient.WithImagePullSecrets(secrets...),
	)
	return adapters.RegistryClient(client), nil
}

// GetClientForPolicy enables bounded live Secret GETs only in the policy's
// namespace. Explicit policy credentials are checked again at this boundary.
func (f *registryClientFactory) GetClientForPolicy(ctx context.Context, creds *kyvernov1.ImageRegistryCredentials, resourceNamespace string, imagePullSecrets []string, policyNamespace string) (engineapi.RegistryClient, error) {
	if policyNamespace == "" {
		return f.GetClient(ctx, creds, resourceNamespace, imagePullSecrets)
	}
	if creds != nil {
		refs, err := kubeutils.ScopeSecretReferences(creds.Secrets, policyNamespace)
		if err != nil {
			return nil, err
		}
		creds = creds.DeepCopy()
		creds.Secrets = refs
	}
	scoped := *f
	if f.secretsLister != nil && creds != nil && len(creds.Secrets) != 0 {
		names := make(map[string]struct{}, len(creds.Secrets))
		for _, ref := range creds.Secrets {
			_, name := kubeutils.ParseSecretReference(ref, policyNamespace)
			names[name] = struct{}{}
		}
		scoped.secretsLister = &policyCredentialLister{SecretLister: f.secretsLister, scoped: kubeutils.ScopeSecretListerWithContext(ctx, f.secretsLister, policyNamespace), namespace: policyNamespace, names: names}
	}
	return scoped.GetClient(ctx, creds, resourceNamespace, imagePullSecrets)
}

type policyCredentialLister struct {
	corev1listers.SecretLister
	scoped    corev1listers.SecretLister
	namespace string
	names     map[string]struct{}
}

func (l *policyCredentialLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	if namespace == l.namespace {
		return &policyCredentialNamespaceLister{SecretNamespaceLister: l.SecretLister.Secrets(namespace), scoped: l.scoped.Secrets(namespace), names: l.names}
	}
	// Resource imagePullSecrets retain their existing informer-only lookup.
	return l.SecretLister.Secrets(namespace)
}

type policyCredentialNamespaceLister struct {
	corev1listers.SecretNamespaceLister
	scoped corev1listers.SecretNamespaceLister
	names  map[string]struct{}
}

func (l *policyCredentialNamespaceLister) Get(name string) (*corev1.Secret, error) {
	if _, ok := l.names[name]; ok {
		return l.scoped.Get(name)
	}
	return l.SecretNamespaceLister.Get(name)
}

func (l *policyCredentialNamespaceLister) GetWithContext(ctx context.Context, name string) (*corev1.Secret, error) {
	if _, ok := l.names[name]; ok {
		if scoped, ok := l.scoped.(interface {
			GetWithContext(context.Context, string) (*corev1.Secret, error)
		}); ok {
			return scoped.GetWithContext(ctx, name)
		}
		return l.scoped.Get(name)
	}
	return l.SecretNamespaceLister.Get(name)
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
