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
