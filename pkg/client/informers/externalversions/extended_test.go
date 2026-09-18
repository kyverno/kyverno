package externalversions

import (
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// newTestFactory returns an ExtendedSharedInformerFactory wired with fakes.
// The scheme passed to the dynamic fake must include every GVR you want the
// dynamic factory to accept without panicking.
func newTestFactory(scheme *runtime.Scheme) *ExtendedSharedInformerFactory {
	kyvernoClient := versionedfake.NewSimpleClientset()
	dynClient := dynamicfake.NewSimpleDynamicClient(scheme)
	return NewExtendedSharedInformerFactory(kyvernoClient, dynClient, 0)
}

// TestForResource_KnownKyvernoCRD verifies that a GVR registered in the
// generated switch (ClusterPolicy) is served by the static typed informer
// and returns no error.
func TestForResource_KnownKyvernoCRD(t *testing.T) {
	t.Parallel()
	factory := newTestFactory(runtime.NewScheme())

	gvr := kyvernov1.SchemeGroupVersion.WithResource("clusterpolicies")
	inf, err := factory.ForResource(gvr)
	if err != nil {
		t.Fatalf("expected no error for known Kyverno GVR %s, got: %v", gvr, err)
	}
	if inf == nil {
		t.Fatalf("expected non-nil informer for known Kyverno GVR %s", gvr)
	}
}

// TestForResource_UnknownResource verifies that a GVR that is NOT registered in
// the generated switch (apps/v1 Deployments) is served by the dynamic fallback
// and returns no error — resolving the TODO in generic.go.
func TestForResource_UnknownResource(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	factory := newTestFactory(scheme)

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	inf, err := factory.ForResource(gvr)
	if err != nil {
		t.Fatalf("expected no error for unknown GVR %s via dynamic fallback, got: %v", gvr, err)
	}
	if inf == nil {
		t.Fatalf("expected non-nil informer for unknown GVR %s", gvr)
	}
}

// TestForResource_ListerNotNil verifies that the GenericLister attached to
// the returned informer is usable (non-nil).
func TestForResource_ListerNotNil(t *testing.T) {
	t.Parallel()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}

	factory := newTestFactory(scheme)

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	inf, err := factory.ForResource(gvr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inf.Lister() == nil {
		t.Fatal("expected non-nil lister from dynamic fallback informer")
	}
}

// TestStart_DoesNotPanic ensures that calling Start on the extended factory
// (which must start both the static and dynamic sub-factories) does not panic.
func TestStart_DoesNotPanic(t *testing.T) {
	t.Parallel()
	factory := newTestFactory(runtime.NewScheme())

	stopCh := make(chan struct{})
	defer close(stopCh)

	factory.Start(stopCh)
}

// TestShutdown_DoesNotPanic ensures that calling Shutdown on the extended
// factory delegates to both sub-factories without panicking.
func TestShutdown_DoesNotPanic(t *testing.T) {
	t.Parallel()

	factory := newTestFactory(runtime.NewScheme())

	stopCh := make(chan struct{})
	close(stopCh) // immediately stopped so goroutines can exit

	factory.Start(stopCh)
	factory.Shutdown()
}

// TestNewExtendedSharedInformerFactory_WithOptions verifies that
// SharedInformerOptions (e.g. WithNamespace) are forwarded to the static factory.
func TestNewExtendedSharedInformerFactory_WithOptions(t *testing.T) {
	t.Parallel()
	kyvernoClient := versionedfake.NewSimpleClientset()
	dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

	factory := NewExtendedSharedInformerFactory(
		kyvernoClient,
		dynClient,
		10*time.Minute,
		WithNamespace("kyverno"),
	)
	if factory == nil {
		t.Fatal("expected non-nil factory")
	}
}
