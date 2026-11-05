package factories

import (
	"context"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
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
	// Qualify resource imagePullSecrets before the namespace falls back to the
	// Kyverno namespace below: that fallback is only for the lister default.
	imagePullSecrets = resourcePullSecretReferences(imagePullSecrets, resourceNamespace)
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

		// create an array of secret names where we will accumulate whats in creds and imagePullSecrets.
		// Policy-authored creds.Secrets may use namespace/name notation and default to the Kyverno
		// namespace; resource imagePullSecrets are already qualified with the resource namespace.
		secrets := make([]string, 0)
		if f.secretsLister != nil && len(creds.Secrets) > 0 {
			secrets = append(secrets, prefixSecretNamespaces(creds.Secrets, config.KyvernoNamespace())...)
		}
		secrets = append(secrets, imagePullSecrets...)

		client := registryclient.New(
			registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
			registryclient.WithImagePullSecrets(secrets...),
			registryclient.WithCredentialHelpers(providers...),
			registryclient.WithAllowInsecureRegistry(creds.AllowInsecureRegistry),
		)
		return adapters.RegistryClient(client), nil
	}

	// creds is nil. create a registry client with only the imagePullSecrets and no providers
	client := registryclient.New(
		registryclient.WithSecretLister(f.secretsLister, resourceNamespace),
		registryclient.WithImagePullSecrets(imagePullSecrets...),
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

// resourcePullSecretReferences qualifies resource-supplied imagePullSecrets names with the
// resource namespace. The resource author, not the policy author, controls these names, and
// the kubelet resolves them only in the Pod's namespace, so they are never read as
// namespace/name notation: a name such as "kyverno/regcred" would otherwise make Kyverno read
// a Secret from a namespace the resource author cannot access. A name that is not a valid
// Secret name (which includes any name containing "/") cannot refer to a Secret and is dropped,
// as is every name when the resource has no namespace. Dropping matches how the keychain
// already treats a Secret that does not exist, and how the kubelet treats a missing pull
// secret, rather than failing admission for a reference that could never resolve.
func resourcePullSecretReferences(names []string, namespace string) []string {
	if len(names) == 0 {
		return nil
	}
	if namespace == "" {
		logging.V(3).Info("ignoring imagePullSecrets of a resource without a namespace", "imagePullSecrets", names)
		return nil
	}
	refs := make([]string, 0, len(names))
	for _, name := range names {
		if errs := validation.IsDNS1123Subdomain(name); len(errs) != 0 {
			logging.V(3).Info("ignoring resource imagePullSecret that is not a valid Secret name", "namespace", namespace, "name", name)
			continue
		}
		refs = append(refs, namespace+"/"+name)
	}
	return refs
}

// prefixSecretNamespaces prefixes each policy-authored secret ref with defaultNamespace unless
// it already uses namespace/name notation. Do not use it for resource imagePullSecrets.
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
