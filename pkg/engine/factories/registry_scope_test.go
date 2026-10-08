package factories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
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

// A documentation IP makes credential resolution independent of DNS. These
// keychain tests use fake Secret clients and never dial the registry.
func scopeRegistrySecret(namespace, user string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "regcred", Namespace: namespace}, Type: corev1.SecretTypeDockerConfigJson,
		Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"192.0.2.1":{"username":"` + user + `","password":"password"}}}`)}}
}

func TestPolicyRegistryFactoryUsesTenantGETAndRetainsOperatorCaches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, policyNamespace, reference, expectedUser string
		forbidden                                      bool
	}{
		{name: "tenant bare credential", policyNamespace: "tenant-a", reference: "regcred", expectedUser: "tenant"},
		{name: "tenant qualified credential", policyNamespace: "tenant-a", reference: "tenant-a/regcred", expectedUser: "tenant"},
		{name: "cluster operator namespace", reference: "operator-registry/regcred", expectedUser: "operator"},
		{name: "foreign reference rejected", policyNamespace: "tenant-a", reference: "operator-registry/regcred"},
		{name: "Forbidden does not fall back", policyNamespace: "tenant-a", reference: "regcred", forbidden: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			require.NoError(t, indexer.Add(scopeRegistrySecret("operator-registry", "operator")))
			require.NoError(t, indexer.Add(scopeRegistrySecret("kyverno", "installation")))
			client := kubefake.NewSimpleClientset(scopeRegistrySecret("tenant-a", "tenant"))
			if tc.forbidden {
				client.PrependReactor("get", "secrets", func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "secrets"}, "regcred", errors.New("denied"))
				})
			}
			lister := kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client.CoreV1())
			factory := DefaultRegistryClientFactory(&mockRegistryClient{}, lister)
			registry, err := engineapi.RegistryClientForPolicy(context.Background(), factory, &kyvernov1.ImageRegistryCredentials{Secrets: []string{tc.reference}}, "tenant-a", nil, tc.policyNamespace)
			if tc.expectedUser == "" && !tc.forbidden {
				require.ErrorContains(t, err, "instead of policy namespace")
				assert.Empty(t, client.Actions())
				return
			}
			require.NoError(t, err)
			keychain, ok := registry.(interface{ Keychain() authn.Keychain })
			require.True(t, ok)
			ref, err := name.ParseReference("192.0.2.1/image:tag")
			require.NoError(t, err)
			authenticator, err := keychain.Keychain().Resolve(ref.Context())
			if tc.forbidden {
				require.True(t, apierrors.IsForbidden(err))
				assert.Nil(t, authenticator)
			} else {
				require.NoError(t, err)
				config, err := authenticator.Authorization()
				require.NoError(t, err)
				assert.Equal(t, tc.expectedUser, config.Username)
			}
			if tc.policyNamespace == "" {
				assert.Empty(t, client.Actions())
			} else {
				require.Len(t, client.Actions(), 1)
				assert.Equal(t, "get", client.Actions()[0].GetVerb())
				assert.Equal(t, "tenant-a", client.Actions()[0].GetNamespace())
			}
		})
	}
}

type registryBlockingSecrets struct {
	corev1client.SecretsGetter
	started chan context.Context
}
type registryBlockingNamespace struct {
	corev1client.SecretInterface
	started chan context.Context
}

func (c *registryBlockingSecrets) Secrets(string) corev1client.SecretInterface {
	return &registryBlockingNamespace{started: c.started}
}
func (c *registryBlockingNamespace) Get(ctx context.Context, _ string, _ metav1.GetOptions) (*corev1.Secret, error) {
	c.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPolicyRegistryFactoryPropagatesCancellation(t *testing.T) {
	t.Parallel()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	client := &registryBlockingSecrets{started: make(chan context.Context, 1)}
	factory := DefaultRegistryClientFactory(&mockRegistryClient{}, kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry, err := engineapi.RegistryClientForPolicy(ctx, factory, &kyvernov1.ImageRegistryCredentials{Secrets: []string{"regcred"}}, "tenant-a", nil, "tenant-a")
	require.NoError(t, err)
	keychain, ok := registry.(interface{ Keychain() authn.Keychain })
	require.True(t, ok)
	ref, err := name.ParseReference("192.0.2.1/image:tag")
	require.NoError(t, err)
	done := make(chan error, 1)
	// Mirror the production remote client, which passes the request context via
	// authn.Resolve. Resolve-only SDK keychains retain the factory-bound context.
	go func() { _, err := authn.Resolve(ctx, keychain.Keychain(), ref.Context()); done <- err }()
	select {
	case apiCtx := <-client.started:
		deadline, ok := apiCtx.Deadline()
		require.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), 5*time.Second)
	case <-time.After(5 * time.Second):
		t.Fatal("Secret GET did not start")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("Secret GET ignored cancellation")
	}
}

func TestPolicyRegistryFactoryKeepsResourcePullSecretsCacheOnly(t *testing.T) {
	t.Parallel()
	for _, explicitCredentials := range []bool{false, true} {
		t.Run(map[bool]string{false: "resource credentials only", true: "with explicit policy credentials"}[explicitCredentials], func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			client := kubefake.NewSimpleClientset(scopeRegistrySecret("tenant-a", "tenant"))
			// The resource's different Secret exists in the API, but is not cached.
			resourceSecret := scopeRegistrySecret("tenant-a", "resource")
			resourceSecret.Name = "resource-regcred"
			_, err := client.CoreV1().Secrets("tenant-a").Create(context.Background(), resourceSecret, metav1.CreateOptions{})
			require.NoError(t, err)
			client.ClearActions()
			var creds *kyvernov1.ImageRegistryCredentials
			if explicitCredentials {
				creds = &kyvernov1.ImageRegistryCredentials{Secrets: []string{"regcred"}}
			}
			factory := DefaultRegistryClientFactory(&mockRegistryClient{}, kubeutils.WithSecretClient(context.Background(), corev1listers.NewSecretLister(indexer), client.CoreV1()))
			registry, err := engineapi.RegistryClientForPolicy(context.Background(), factory, creds, "tenant-a", []string{"resource-regcred"}, "tenant-a")
			require.NoError(t, err)
			keychain, ok := registry.(interface{ Keychain() authn.Keychain })
			require.True(t, ok)
			ref, err := name.ParseReference("192.0.2.1/image:tag")
			require.NoError(t, err)
			authenticator, err := keychain.Keychain().Resolve(ref.Context())
			require.NoError(t, err)
			config, err := authenticator.Authorization()
			require.NoError(t, err)
			if explicitCredentials {
				assert.Equal(t, "tenant", config.Username)
				require.Len(t, client.Actions(), 1)
				get, ok := client.Actions()[0].(k8stesting.GetAction)
				require.True(t, ok)
				assert.Equal(t, "regcred", get.GetName())
			} else {
				assert.Equal(t, "", config.Username)
				assert.Empty(t, client.Actions())
			}
		})
	}
}
