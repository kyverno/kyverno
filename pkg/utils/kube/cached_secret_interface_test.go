package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCachedSecretInterfaceGet(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kyverno",
			Name:      "registry-credentials",
		},
		Data: map[string][]byte{
			".dockerconfigjson": []byte("secret-data"),
		},
	}
	client := fake.NewSimpleClientset(secret)
	factory := informers.NewSharedInformerFactory(client, 0)
	err := factory.Core().V1().Secrets().Informer().GetIndexer().Add(secret)
	require.NoError(t, err)

	secrets := NewCachedSecretInterface(factory.Core().V1().Secrets().Lister(), "kyverno")
	got, err := secrets.Get(context.Background(), "registry-credentials", metav1.GetOptions{})

	require.NoError(t, err)
	assert.Equal(t, "registry-credentials", got.Name)
	assert.Equal(t, []byte("secret-data"), got.Data[".dockerconfigjson"])
}

func TestCachedSecretInterfaceGetNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()
	factory := informers.NewSharedInformerFactory(client, 0)

	secrets := NewCachedSecretInterface(factory.Core().V1().Secrets().Lister(), "kyverno")
	_, err := secrets.Get(context.Background(), "missing", metav1.GetOptions{})

	assert.True(t, apierrors.IsNotFound(err))
}

func TestCachedSecretInterfaceList(t *testing.T) {
	objects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kyverno",
				Name:      "included",
				Labels: map[string]string{
					"registry": "true",
				},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "kyverno",
				Name:      "excluded",
				Labels: map[string]string{
					"registry": "false",
				},
			},
		},
	}
	client := fake.NewSimpleClientset(objects...)
	factory := informers.NewSharedInformerFactory(client, 0)
	for _, object := range objects {
		err := factory.Core().V1().Secrets().Informer().GetIndexer().Add(object)
		require.NoError(t, err)
	}

	secrets := NewCachedSecretInterface(factory.Core().V1().Secrets().Lister(), "kyverno")
	got, err := secrets.List(context.Background(), metav1.ListOptions{LabelSelector: "registry=true"})

	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	assert.Equal(t, "included", got.Items[0].Name)
}
