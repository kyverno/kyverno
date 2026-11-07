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
	kubefake "k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestSecretKeychainRefreshesTenantCredentialsAndKeepsUnscopedCacheOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "registry", Namespace: "tenant"}, Type: corev1.SecretTypeDockerConfigJson}
	secret.Data = map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"192.0.2.1":{"username":"first","password":"test"}}}`)}
	client := kubefake.NewSimpleClientset(secret)
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	lister := kubeutils.WithSecretClient(ctx, corev1listers.NewSecretLister(indexer), client.CoreV1())
	resource := name.MustParseReference("192.0.2.1/image:latest").Context()
	unscoped := NewSecretsKeychain(lister, "tenant", logr.Discard(), "registry")
	authenticator, err := authn.Resolve(ctx, unscoped, resource)
	require.NoError(t, err)
	assert.Equal(t, authn.Anonymous, authenticator)
	assert.Empty(t, client.Actions(), "unscoped consumers must not GET live tenant Secrets")
	scoped := NewSecretsKeychain(kubeutils.ScopeSecretLister(lister, "tenant"), "tenant", logr.Discard(), "registry")
	check := func(expected string) {
		authenticator, err := authn.Resolve(ctx, scoped, resource)
		require.NoError(t, err)
		config, err := authenticator.Authorization()
		require.NoError(t, err)
		assert.Equal(t, expected, config.Username)
	}
	check("first")
	rotated := secret.DeepCopy()
	rotated.Data[corev1.DockerConfigJsonKey] = []byte(`{"auths":{"192.0.2.1":{"username":"second","password":"test"}}}`)
	_, err = client.CoreV1().Secrets("tenant").Update(ctx, rotated, metav1.UpdateOptions{})
	require.NoError(t, err)
	check("second")
	gets := 0
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" {
			gets++
		}
		assert.NotEqual(t, "list", action.GetVerb())
	}
	assert.Equal(t, 2, gets)
}
