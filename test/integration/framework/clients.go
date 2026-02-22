package framework

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewClients creates real Kubernetes and Kyverno clientsets backed by the
// envtest API server — replacing the fake.NewSimpleClientset() pattern.
func NewClients(t *testing.T, cfg *rest.Config) (kubernetes.Interface, versioned.Interface) {
	t.Helper()
	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create kubernetes client: %v", err)
	}
	kyvernoClient, err := versioned.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("failed to create kyverno client: %v", err)
	}
	return kubeClient, kyvernoClient
}
