package framework

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewNamespaceResolver_ResolvesNamespacesWithTheirLabels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := fake.NewClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "payments-prod", Labels: map[string]string{"environment": "production"}},
	})

	resolve := NewNamespaceResolver(ctx, client)

	ns := resolve("payments-prod")
	require.NotNil(t, ns)
	assert.Equal(t, "production", ns.Labels["environment"])
	assert.Nil(t, resolve("does-not-exist"), "an unknown namespace must resolve to nil, as in production")
}

func TestNewNamespaceResolver_ResolvesANamespaceCreatedAfterItStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := fake.NewClientset()
	resolve := NewNamespaceResolver(ctx, client)

	_, err := client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-acme", Labels: map[string]string{"tenant.example.com/isolation": "strict"}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		ns := resolve("tenant-acme")
		return ns != nil && ns.Labels["tenant.example.com/isolation"] == "strict"
	}, 5*time.Second, 50*time.Millisecond, "a namespace created after the resolver started must resolve")
}

func TestCreateNamespaceWithLabels_KeepsTheMetadataNameLabel(t *testing.T) {
	client := fake.NewClientset()

	CreateNamespaceWithLabels(t, client, "orders", map[string]string{"team": "checkout"})

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), "orders", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"kubernetes.io/metadata.name": "orders", "team": "checkout"}, ns.Labels)
}

func TestCreateNamespaceWithLabels_MetadataNameAlwaysMatchesTheNamespace(t *testing.T) {
	client := fake.NewClientset()

	CreateNamespaceWithLabels(t, client, "orders", map[string]string{"kubernetes.io/metadata.name": "something-else"})

	ns, err := client.CoreV1().Namespaces().Get(context.Background(), "orders", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "orders", ns.Labels["kubernetes.io/metadata.name"])
}
