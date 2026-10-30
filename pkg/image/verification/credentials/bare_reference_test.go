package credentials

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// ScopePolicy canonicalizes a namespaced policy's references to namespace/name
// before they ever reach the keychain, so the default namespace is unreachable
// on that path. This asserts the layer underneath it independently: given a
// bare name and a lister confined to the policy namespace, the credential
// namespace derived from that lister resolves the Secret in the policy's own
// namespace rather than Kyverno's.
//
// Without this, a future path that reaches the keychain without canonicalizing
// first silently reads the installation namespace again, which was the original
// bug.
func TestBareReferenceResolvesInThePolicyNamespace(t *testing.T) {
	t.Parallel()
	const installation = "kyverno"
	resource := name.MustParseReference("192.0.2.1/image:latest").Context()
	dockerConfig := func(username string) map[string][]byte {
		return map[string][]byte{
			corev1.DockerConfigJsonKey: []byte(`{"auths":{"192.0.2.1":{"username":"` + username + `","password":"test"}}}`),
		}
	}

	for _, tc := range []struct {
		name, scope, expected string
	}{
		{name: "namespaced policy reads its own namespace", scope: "tenant", expected: "tenant-creds"},
		{name: "cluster scoped policy keeps the installation namespace", scope: "", expected: "installation-creds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			require.NoError(t, indexer.Add(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "tenant"},
				Type:       corev1.SecretTypeDockerConfigJson,
				Data:       dockerConfig("tenant-creds"),
			}))
			require.NoError(t, indexer.Add(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: installation},
				Type:       corev1.SecretTypeDockerConfigJson,
				Data:       dockerConfig("installation-creds"),
			}))

			lister := kubeutils.ScopeSecretLister(corev1listers.NewSecretLister(indexer), tc.scope)
			namespace := kubeutils.CredentialNamespace(lister, installation)
			// A bare name, deliberately not canonicalized by ScopePolicy.
			keychain := NewSecretsKeychain(lister, namespace, logr.Discard(), "registry")

			authenticator, err := authn.Resolve(context.Background(), keychain, resource)
			require.NoError(t, err)
			config, err := authenticator.Authorization()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, config.Username)
		})
	}
}
