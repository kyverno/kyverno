package globalcontext

import (
	"context"
	"testing"
	"time"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

type discardEvents struct{}

func (discardEvents) Add(...event.Info) {}

// Exercise the informer -> queue -> reconcile -> backing informer -> store path.
// Creating the GCE only after startup is essential to reproducing #15828.
func TestController_LoadsGlobalContextEntryCreatedAfterStartup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	gvr := schema.GroupVersionResource{Group: "example.io", Version: "v1alpha1", Resource: "widgets"}
	object := func(name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "example.io/v1alpha1",
			"kind":       "Widget",
			"metadata": map[string]interface{}{
				"name": name, "namespace": "default",
			},
			"spec": map[string]interface{}{"value": name},
		}}
	}
	a := object("a")
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "WidgetList"}, a,
	)
	listed, watched := make(chan struct{}, 1), make(chan struct{}, 1)
	dyn.PrependReactor("list", gvr.Resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
		if action.GetResource() == gvr {
			select {
			case listed <- struct{}{}:
			default:
			}
		}
		return false, nil, nil
	})
	dyn.PrependWatchReactor(gvr.Resource, func(action clienttesting.Action) (bool, watch.Interface, error) {
		if action.GetResource() == gvr {
			select {
			case watched <- struct{}{}:
			default:
			}
		}
		return false, nil, nil
	})
	kube := kubefake.NewSimpleClientset()
	kyverno := kyvernofake.NewSimpleClientset()
	factory := kyvernoinformers.NewSharedInformerFactory(kyverno, 0)
	informer := factory.Kyverno().V2beta1().GlobalContextEntries()
	storage := store.New(0)
	ctrl := NewController(informer, kube, dclient.NewFakeClientWithDisco(dyn, kube, nil), kyverno,
		storage, discardEvents{}, 0, 0, false, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctrl.Run(ctx, Workers)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("global-context controller did not stop")
		}
		storage.Delete("runtime-widgets")
		factory.Shutdown()
	})
	factory.Start(ctx.Done())
	require.True(t, cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced))
	require.Empty(t, dyn.Actions(), "backing resource must not be requested before GCE creation")
	_, exists := storage.Get("runtime-widgets")
	require.False(t, exists)

	_, err := kyverno.KyvernoV2beta1().GlobalContextEntries().Create(ctx, &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "runtime-widgets"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{KubernetesResource: &kyvernov2beta1.KubernetesResource{
			Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource, Namespace: "default",
		}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	for _, request := range []struct {
		name string
		done <-chan struct{}
	}{{"LIST", listed}, {"WATCH", watched}} {
		select {
		case <-request.done:
		case <-ctx.Done():
			t.Fatalf("backing GVR %s was not requested: %v", request.name, ctx.Err())
		}
	}

	var loaded store.Entry
	require.Eventually(t, func() bool {
		loaded, exists = storage.Get("runtime-widgets")
		return exists
	}, 5*time.Second, 10*time.Millisecond, "runtime-created GCE must become available without restart")
	data, err := loaded.Get("")
	require.NoError(t, err)
	require.Equal(t, []interface{}{a.Object}, data)

	// A subsequent backing-resource event must update the same entry, proving
	// that WATCH is functional rather than just recorded as an attempted call.
	b := object("b")
	_, err = dyn.Resource(gvr).Namespace("default").Create(ctx, b, metav1.CreateOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		data, err := loaded.Get("")
		if err != nil {
			return false
		}
		objects, ok := data.([]interface{})
		return ok && len(objects) == 2
	}, 5*time.Second, 10*time.Millisecond)
	data, err = loaded.Get("")
	require.NoError(t, err)
	require.ElementsMatch(t, []interface{}{a.Object, b.Object}, data)
	current, exists := storage.Get("runtime-widgets")
	require.True(t, exists)
	require.Same(t, loaded, current, "backing events must not recreate the GCE entry")
}
