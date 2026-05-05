package cluster

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newWrappedClient(t *testing.T, objs ...runtime.Object) dclient.Interface {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	gvrToListKind := map[schema.GroupVersionResource]string{
		{
			Group:    "",
			Version:  "v1",
			Resource: "secrets",
		}: "SecretList",
	}

	real, err := dclient.NewFakeClient(scheme, gvrToListKind, objs...)
	require.NoError(t, err)

	real.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	return NewWrapper(real)
}

func TestWrapper_CreateResource_UsesFakeClient(t *testing.T) {
	client := newWrappedClient(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	_, err := client.CreateResource(
		context.TODO(),
		"v1",
		"Secret",
		"default",
		secret,
		false,
	)
	assert.NoError(t, err)

	// Must be readable after fake write
	got, err := client.GetResource(
		context.TODO(),
		"v1",
		"Secret",
		"default",
		"test-secret",
	)
	assert.NoError(t, err)
	assert.Equal(t, "test-secret", got.GetName())
}

func TestWrapper_FakeOverridesReal(t *testing.T) {
	realSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "real-secret",
			Namespace: "default",
		},
	}

	client := newWrappedClient(t, realSecret)

	// Fake-write a different object with same name
	fakeSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "real-secret",
			Namespace: "default",
			Labels: map[string]string{
				"source": "fake",
			},
		},
	}

	_, err := client.CreateResource(
		context.TODO(),
		"v1",
		"Secret",
		"default",
		fakeSecret,
		false,
	)
	assert.NoError(t, err)

	got, err := client.GetResource(
		context.TODO(),
		"v1",
		"Secret",
		"default",
		"real-secret",
	)
	assert.NoError(t, err)

	assert.Equal(t, "fake", got.GetLabels()["source"])
}

func TestWrapper_GetResource_FallsBackToReal(t *testing.T) {
	realSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "real-secret",
			Namespace: "default",
		},
	}

	client := newWrappedClient(t, realSecret)

	got, err := client.GetResource(
		context.TODO(),
		"v1",
		"Secret",
		"default",
		"real-secret",
	)

	require.NoError(t, err)
	assert.Equal(t, "real-secret", got.GetName())
}
