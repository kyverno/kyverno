package testutil

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// NewFakeClient creates a fake controller-runtime client with Kyverno schemes
func NewFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()

	// Add Kyverno v1 and v2 schemes
	_ = kyvernov1.AddToScheme(scheme)
	_ = kyvernov2.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

// NewFakeClientWithStatus creates a fake client with status subresource support
func NewFakeClientWithStatus(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = kyvernov1.AddToScheme(scheme)
	_ = kyvernov2.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithStatusSubresource(objects...).
		Build()
}

// NewKyvernoFakeClient creates a fake Kyverno versioned clientset
// Drop-in replacement for versionedfake.NewSimpleClientset()
func NewKyvernoFakeClient(objects ...runtime.Object) kyvernoclient.Interface {
	return kyvernofake.NewSimpleClientset(objects...)
}
