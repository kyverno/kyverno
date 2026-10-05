package informers

import (
	"context"
	"errors"
	"testing"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
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

var deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

func newDeployment(namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}}
}

func newDeploymentScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add appsv1 to scheme: %v", err)
	}
	return scheme
}

// TestLifecycle_FallbackInformerStartsSyncsAndStops registers a fallback informer and
// asserts that Start populates its cache, WaitForCacheSync covers it, and Shutdown returns.
func TestLifecycle_FallbackInformerStartsSyncsAndStops(t *testing.T) {
	t.Parallel()
	dynClient := dynamicfake.NewSimpleDynamicClient(newDeploymentScheme(t), newDeployment("default", "d1"))
	factory := NewExtendedSharedInformerFactory(versionedfake.NewSimpleClientset(), dynClient, 0)

	inf, err := factory.ForResource(deploymentsGVR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stopCh := make(chan struct{})
	factory.Start(stopCh)

	synced := factory.WaitForCacheSync(stopCh)
	if ok, found := synced[dynamicInformerType]; !found || !ok {
		t.Fatalf("expected dynamic informers to be reported as synced, got %v", synced)
	}
	objs, err := inf.Lister().List(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object in the fallback informer cache, got %d", len(objs))
	}

	close(stopCh)
	done := make(chan struct{})
	go func() {
		factory.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return, dynamic informers were not stopped")
	}
}

// TestWaitForCacheSync_FallbackInitialListFails makes the initial list fail and asserts
// that WaitForCacheSync reports the dynamic informers as not synced.
func TestWaitForCacheSync_FallbackInitialListFails(t *testing.T) {
	t.Parallel()
	dynClient := dynamicfake.NewSimpleDynamicClient(newDeploymentScheme(t))
	dynClient.PrependReactor("list", "deployments", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("boom")
	})
	factory := NewExtendedSharedInformerFactory(versionedfake.NewSimpleClientset(), dynClient, 0)
	if _, err := factory.ForResource(deploymentsGVR); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	factory.Start(ctx.Done())

	synced := factory.WaitForCacheSync(ctx.Done())
	if ok, found := synced[dynamicInformerType]; !found || ok {
		t.Fatalf("expected dynamic informers to be reported as NOT synced, got %v", synced)
	}
	factory.Shutdown()
}

// TestWithNamespace_AppliesToFallback asserts the namespace filter reaches the dynamic factory.
func TestWithNamespace_AppliesToFallback(t *testing.T) {
	t.Parallel()
	dynClient := dynamicfake.NewSimpleDynamicClient(newDeploymentScheme(t),
		newDeployment("kyverno", "in-scope"),
		newDeployment("other", "out-of-scope"),
	)
	factory := NewExtendedSharedInformerFactory(versionedfake.NewSimpleClientset(), dynClient, 0, WithNamespace("kyverno"))
	inf, err := factory.ForResource(deploymentsGVR)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stopCh := make(chan struct{})
	defer func() {
		close(stopCh)
		factory.Shutdown()
	}()
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	objs, err := inf.Lister().List(labels.Everything())
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected only the object in namespace kyverno, got %d", len(objs))
	}
}

// TestWithTweakListOptions_AppliesToFallback asserts the list-option tweak reaches the dynamic factory.
func TestWithTweakListOptions_AppliesToFallback(t *testing.T) {
	t.Parallel()
	dynClient := dynamicfake.NewSimpleDynamicClient(newDeploymentScheme(t))
	selectors := make(chan string, 8)
	dynClient.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
		if la, ok := action.(clienttesting.ListAction); ok {
			select {
			case selectors <- la.GetListRestrictions().Labels.String():
			default:
			}
		}
		return false, nil, nil
	})
	factory := NewExtendedSharedInformerFactory(versionedfake.NewSimpleClientset(), dynClient, 0,
		WithTweakListOptions(func(o *metav1.ListOptions) { o.LabelSelector = "app=kyverno" }),
	)
	if _, err := factory.ForResource(deploymentsGVR); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stopCh := make(chan struct{})
	defer func() {
		close(stopCh)
		factory.Shutdown()
	}()
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	select {
	case got := <-selectors:
		if got != "app=kyverno" {
			t.Fatalf("expected label selector app=kyverno on the dynamic list, got %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dynamic informer never listed")
	}
}
