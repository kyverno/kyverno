package testutil

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewFakeClient creates a fake Kubernetes client with Kyverno schemes registered
func NewFakeClient(objects ...client.Object) client.Client {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = kyvernov1.AddToScheme(s)
	_ = kyvernov2.AddToScheme(s)

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		Build()
}

// NewFakeClientWithStatus creates a fake client that also handles status subresources
func NewFakeClientWithStatus(objects ...client.Object) client.Client {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = kyvernov1.AddToScheme(s)
	_ = kyvernov2.AddToScheme(s)

	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		WithStatusSubresource(objects...).
		Build()
}
