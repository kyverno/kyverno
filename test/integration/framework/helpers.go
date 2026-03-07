package framework

import (
	"context"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateClusterPolicy creates a ClusterPolicy in the envtest API server
// and registers a cleanup function to delete it when the test finishes.
func CreateClusterPolicy(t *testing.T, client versioned.Interface, policy *kyvernov1.ClusterPolicy) *kyvernov1.ClusterPolicy {
	t.Helper()
	created, err := client.KyvernoV1().ClusterPolicies().Create(
		context.Background(), policy, metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.KyvernoV1().ClusterPolicies().Delete(
			context.Background(), created.Name, metav1.DeleteOptions{})
	})
	return created
}

// UniqueNamespace creates a per-test namespace for isolation and registers
// cleanup to delete it when the test finishes.
func UniqueNamespace(t *testing.T, kubeClient kubernetes.Interface) *corev1.Namespace {
	t.Helper()
	name := strings.ToLower(strings.ReplaceAll(t.Name(), "/", "-"))
	if len(name) > 63 {
		name = name[:63]
	}
	ns, err := kubeClient.CoreV1().Namespaces().Create(context.Background(),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}},
		metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = kubeClient.CoreV1().Namespaces().Delete(
			context.Background(), name, metav1.DeleteOptions{})
	})
	return ns
}
