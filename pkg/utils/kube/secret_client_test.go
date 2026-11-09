package kube

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestScopedSecretListerPreservesCachesAndUsesAuthorizedTenantGET(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, namespace                    string
		scoped, cached, forbidden, foreign bool
	}{
		{name: "installation cache", namespace: "kyverno", cached: true},
		{name: "operator cache", namespace: "operator-registry", cached: true},
		{name: "tenant cache", namespace: "tenant-a", cached: true, scoped: true},
		{name: "tenant live GET", namespace: "tenant-a", scoped: true},
		{name: "unscoped remains cache only", namespace: "tenant-a"},
		{name: "Forbidden stops lookup", namespace: "tenant-a", scoped: true, forbidden: true},
		{name: "foreign namespace rejected", namespace: "tenant-b", scoped: true, foreign: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			cached := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: tc.namespace}, Data: map[string][]byte{"source": []byte("cache")}}
			if tc.cached {
				require.NoError(t, indexer.Add(cached))
			}
			live := cached.DeepCopy()
			live.Data["source"] = []byte("live")
			client := kubefake.NewSimpleClientset(live)
			if tc.forbidden {
				client.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "registry", errors.New("tenant credentials denied"))
				})
			}
			lister := WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client.CoreV1())
			if tc.scoped {
				lister = ScopeSecretLister(lister, "tenant-a")
			}
			result, err := lister.Secrets(tc.namespace).Get("registry")
			if tc.foreign {
				require.True(t, apierrors.IsForbidden(err))
				assert.Empty(t, client.Actions())
				return
			}
			if tc.forbidden {
				require.True(t, apierrors.IsForbidden(err))
			} else if !tc.scoped && !tc.cached {
				require.True(t, apierrors.IsNotFound(err))
				assert.Empty(t, client.Actions())
				return
			} else {
				require.NoError(t, err)
				if tc.cached {
					assert.Equal(t, "cache", string(result.Data["source"]))
				} else {
					assert.Equal(t, "live", string(result.Data["source"]))
				}
			}
			if tc.cached {
				assert.Empty(t, client.Actions())
			} else {
				require.Len(t, client.Actions(), 1)
				assert.Equal(t, "get", client.Actions()[0].GetVerb())
				assert.Equal(t, "tenant-a", client.Actions()[0].GetNamespace())
			}
		})
	}
}

// A lister that never went through WithSecretClient still has to confine
// lookups. Returning it unscoped would silently drop the namespace boundary for
// any future caller that forgets the wrapper, which is the one failure mode a
// security helper must not have.
func TestScopeSecretListerFailsClosedWithoutTheClientWrapper(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, scope, lookup string
		forbidden, found    bool
	}{
		{name: "own namespace resolves from cache", scope: "tenant-a", lookup: "tenant-a", found: true},
		{name: "foreign namespace is denied", scope: "tenant-a", lookup: "tenant-b", forbidden: true},
		{name: "installation namespace is denied", scope: "tenant-a", lookup: "kyverno", forbidden: true},
		{name: "empty scope leaves the lister alone", scope: "", lookup: "kyverno", found: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			for _, namespace := range []string{"tenant-a", "tenant-b", "kyverno"} {
				require.NoError(t, indexer.Add(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: namespace}}))
			}
			// A plain informer lister, exactly what the CLI passes.
			plain := corev1listers.NewSecretLister(indexer)
			scoped := ScopeSecretLister(plain, tc.scope)
			require.NotNil(t, scoped)
			secret, err := scoped.Secrets(tc.lookup).Get("registry")
			if tc.forbidden {
				require.True(t, apierrors.IsForbidden(err), "expected Forbidden, got %v", err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.lookup, secret.Namespace)
		})
	}
}

// A nil lister stays nil: NewSecretsKeychain already fails closed on it, and
// turning it into a deny-all wrapper would change the error callers see.
func TestScopeSecretListerLeavesNilAlone(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ScopeSecretLister(nil, "tenant-a"))
	assert.Nil(t, ScopeSecretListerWithContext(context.Background(), nil, "tenant-a"))
}

// compilerImpl.Compile scopes the lister and then hands the result to
// NewIvFuncs, which scopes it again for the same namespace. If that second call
// wrapped instead of returning the existing scope, the outer lister would
// delegate through the inner one's plain Get and the authorized GET would run
// under the process context rather than the admission request's - so a
// cancelled request would no longer abort the API call.
func TestDoubleScopingKeepsTheRequestContext(t *testing.T) {
	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	client := &blockingScopeSecretClient{started: make(chan context.Context, 1)}
	lister := WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	once := ScopeSecretListerWithContext(ctx, lister, "tenant-a")
	// The second scope, exactly as NewIvFuncs applies it.
	twice := ScopeSecretLister(once, "tenant-a")
	assert.Same(t, once, twice, "re-scoping the same namespace must not wrap again")

	finished := make(chan error, 1)
	go func() { _, err := twice.Secrets("tenant-a").Get("registry"); finished <- err }()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("credential GET did not start")
	}
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled, "the live GET must observe the request context")
	case <-time.After(time.Second):
		t.Fatal("double-scoped credential GET ignored cancellation")
	}
}

func TestCredentialNamespace(t *testing.T) {
	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	plain := corev1listers.NewSecretLister(indexer)
	wrapped := WithSecretClient(context.Background(), plain, kubefake.NewSimpleClientset().CoreV1())

	t.Run("confined lister reports its own namespace", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "tenant-a", CredentialNamespace(ScopeSecretLister(wrapped, "tenant-a"), "kyverno"))
		assert.Equal(t, "tenant-a", CredentialNamespace(ScopeSecretLister(plain, "tenant-a"), "kyverno"))
		assert.Equal(t, "tenant-a", CredentialNamespace(ScopeSecretListerWithContext(context.Background(), wrapped, "tenant-a"), "kyverno"))
	})

	t.Run("cluster scoped falls back to the installation namespace", func(t *testing.T) {
		t.Parallel()
		// An empty scope is a cluster-scoped policy, which must keep resolving
		// bare names in Kyverno's namespace.
		assert.Equal(t, "kyverno", CredentialNamespace(ScopeSecretLister(wrapped, ""), "kyverno"))
		assert.Equal(t, "kyverno", CredentialNamespace(plain, "kyverno"))
		assert.Equal(t, "kyverno", CredentialNamespace(nil, "kyverno"))
	})
}

type blockingScopeSecretClient struct {
	corev1client.SecretsGetter
	started chan context.Context
}
type blockingScopeNamespaceClient struct {
	corev1client.SecretInterface
	started chan context.Context
}

func (c *blockingScopeSecretClient) Secrets(string) corev1client.SecretInterface {
	return &blockingScopeNamespaceClient{started: c.started}
}
func (c *blockingScopeNamespaceClient) Get(ctx context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
	c.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestScopedSecretListerBindsRequestContext(t *testing.T) {
	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	client := &blockingScopeSecretClient{started: make(chan context.Context, 1)}
	lister := WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scoped := ScopeSecretListerWithContext(ctx, lister, "tenant-a")
	finished := make(chan error, 1)
	go func() { _, err := scoped.Secrets("tenant-a").Get("registry"); finished <- err }()
	select {
	case requestCtx := <-client.started:
		deadline, ok := requestCtx.Deadline()
		require.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("credential GET did not start")
	}
	cancel()
	select {
	case err := <-finished:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("credential GET ignored cancellation")
	}
}
