package registryclient

import (
	"context"
	"testing"

	kyvernoconfig "github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type trackingSecretLister struct {
	corev1listers.SecretLister
	accessed map[string]bool
}

func (t *trackingSecretLister) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &trackingSecretNamespaceLister{
		SecretNamespaceLister: t.SecretLister.Secrets(namespace),
		accessed:              t.accessed,
		namespace:             namespace,
	}
}

type trackingSecretNamespaceLister struct {
	corev1listers.SecretNamespaceLister
	accessed  map[string]bool
	namespace string
}

func (t *trackingSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	t.accessed[t.namespace+"/"+name] = true
	return t.SecretNamespaceLister.Get(name)
}

func startSecretInformer(t *testing.T, secrets ...runtime.Object) corev1listers.SecretLister {
	t.Helper()
	clientset := fake.NewSimpleClientset(secrets...)
	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	stopCh := make(chan struct{})
	t.Cleanup(func() { close(stopCh) })
	secretInformer := informerFactory.Core().V1().Secrets().Informer()
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)
	for _, obj := range secrets {
		require.NoError(t, secretInformer.GetIndexer().Add(obj))
	}
	return informerFactory.Core().V1().Secrets().Lister()
}

func TestSecretInterfaceFromLister_Get(t *testing.T) {
	kyvernoNS := kyvernoconfig.KyvernoNamespace()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: kyvernoNS,
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte(`{"auths":{}}`),
		},
	}
	lister := startSecretInformer(t, secret)
	trackingLister := &trackingSecretLister{
		SecretLister: lister,
		accessed:     make(map[string]bool),
	}
	secretInterface := SecretInterfaceFromLister(trackingLister, kyvernoNS)

	got, err := secretInterface.Get(context.Background(), "registry-creds", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, secret.Name, got.Name)
	assert.Equal(t, secret.Namespace, got.Namespace)
	assert.True(t, trackingLister.accessed[kyvernoNS+"/registry-creds"])
}

func TestSecretInterfaceFromLister_GetWithNamespaceReference(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-creds",
			Namespace: "other-ns",
		},
	}
	lister := startSecretInformer(t, secret)
	trackingLister := &trackingSecretLister{
		SecretLister: lister,
		accessed:     make(map[string]bool),
	}
	secretInterface := SecretInterfaceFromLister(trackingLister, kyvernoconfig.KyvernoNamespace())

	got, err := secretInterface.Get(context.Background(), "other-ns/other-creds", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, secret.Name, got.Name)
	assert.True(t, trackingLister.accessed["other-ns/other-creds"])
}

func TestSecretInterfaceFromLister_GetNotFound(t *testing.T) {
	lister := startSecretInformer(t)
	secretInterface := SecretInterfaceFromLister(lister, kyvernoconfig.KyvernoNamespace())

	_, err := secretInterface.Get(context.Background(), "missing-secret", metav1.GetOptions{})
	require.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestSecretInterfaceFromLister_NilLister(t *testing.T) {
	secretInterface := SecretInterfaceFromLister(nil, kyvernoconfig.KyvernoNamespace())

	_, err := secretInterface.Get(context.Background(), "registry-creds", metav1.GetOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret lister is nil")
}

func TestCachedSecretInterface(t *testing.T) {
	kyvernoNS := kyvernoconfig.KyvernoNamespace()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: kyvernoNS,
		},
	}
	lister := startSecretInformer(t, secret)
	secretInterface := CachedSecretInterface(lister)

	got, err := secretInterface.Get(context.Background(), "registry-creds", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, secret.Name, got.Name)
}
