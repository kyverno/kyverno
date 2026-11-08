package internal

import (
	"context"
	"testing"

	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// setupRegistryClient hands the same lister to namespaced ivpol credential resolution and to every
// other secret consumer (the legacy image-verification engine, the shared CEL context provider,
// resource imagePullSecrets). Wrapping multiLister with WithSecretClient must therefore not widen
// secret access on its own: only an explicit ScopeSecretLister may reach the live API, and only
// inside the policy's own namespace. See pkg/utils/kube/AGENTS.md.
func TestRegistrySecretListerKeepsCacheOnlyDefault(t *testing.T) {
	t.Parallel()
	const installNamespace, policyNamespace = "kyverno", "tenant-a"

	newLister := func(t *testing.T) (corev1listers.SecretLister, *kubefake.Clientset) {
		t.Helper()
		indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		cached := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "regcred", Namespace: installNamespace},
			Data:       map[string][]byte{"source": []byte("cache")},
		}
		require.NoError(t, indexer.Add(cached))
		live := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "regcred", Namespace: policyNamespace},
			Data:       map[string][]byte{"source": []byte("live")},
		}
		client := kubefake.NewSimpleClientset(live)
		ms := &multiLister{listersMap: map[string]corev1listers.SecretLister{
			installNamespace: corev1listers.NewSecretLister(indexer),
		}}
		return kubeutils.WithSecretClient(context.Background(), ms, client.CoreV1()), client
	}

	t.Run("configured namespace still resolves from the informer", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		secret, err := lister.Secrets(installNamespace).Get("regcred")
		require.NoError(t, err)
		assert.Equal(t, "cache", string(secret.Data["source"]))
		assert.Empty(t, client.Actions(), "a cache hit must not reach the API server")
	})

	t.Run("unconfigured namespace stays NotFound without an API call", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := lister.Secrets(policyNamespace).Get("regcred")
		require.True(t, apierrors.IsNotFound(err), "unscoped lookups must keep multiLister behavior, got %v", err)
		assert.Empty(t, client.Actions(), "unscoped consumers must never trigger a tenant secret read")
	})

	t.Run("scoped lookup reaches the policy namespace", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		secret, err := kubeutils.ScopeSecretLister(lister, policyNamespace).Secrets(policyNamespace).Get("regcred")
		require.NoError(t, err)
		assert.Equal(t, "live", string(secret.Data["source"]))
		require.Len(t, client.Actions(), 1)
		assert.Equal(t, "get", client.Actions()[0].GetVerb())
		assert.Equal(t, policyNamespace, client.Actions()[0].GetNamespace())
	})

	t.Run("scoped lookup rejects the installation namespace", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := kubeutils.ScopeSecretLister(lister, policyNamespace).Secrets(installNamespace).Get("regcred")
		require.True(t, apierrors.IsForbidden(err), "a namespaced policy must not borrow installation credentials, got %v", err)
		assert.Empty(t, client.Actions())
	})

	t.Run("cluster scoped policies are left unwrapped", func(t *testing.T) {
		t.Parallel()
		lister, client := newLister(t)
		_, err := kubeutils.ScopeSecretLister(lister, "").Secrets(policyNamespace).Get("regcred")
		require.True(t, apierrors.IsNotFound(err), "an empty namespace must not enable live reads, got %v", err)
		assert.Empty(t, client.Actions())
	})
}
