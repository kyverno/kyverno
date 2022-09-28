package metrics

import (
	"context"

	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type Recorder interface {
	Record(clientQueryOperation ClientQueryOperation)
}

type clientQueryRecorder struct {
	manager MetricsConfigManager
	ns      string
	kind    string
	client  ClientType
}

func NamespacedClientQueryRecorder(m MetricsConfigManager, ns, kind string, client ClientType) Recorder {
	return &clientQueryRecorder{
		manager: m,
		ns:      ns,
		kind:    kind,
		client:  client,
	}
}

func ClusteredClientQueryRecorder(m MetricsConfigManager, kind string, client ClientType) Recorder {
	return &clientQueryRecorder{
		manager: m,
		kind:    kind,
		client:  client,
	}
}

func (r *clientQueryRecorder) Record(clientQueryOperation ClientQueryOperation) {
	r.manager.RecordClientQueries(clientQueryOperation, r.client, r.kind, r.ns)
}

type objectClient[T metav1.Object] struct {
	recorder Recorder
	inner    controllerutils.ObjectClient[T]
}

func (c *objectClient[T]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	defer c.recorder.Record(ClientCreate)
	return c.inner.Create(ctx, obj, opts)
}

func (c *objectClient[T]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	defer c.recorder.Record(ClientUpdate)
	return c.inner.Update(ctx, obj, opts)
}

func (c *objectClient[T]) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	defer c.recorder.Record(ClientDelete)
	return c.inner.Delete(ctx, name, opts)
}

func (c *objectClient[T]) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	defer c.recorder.Record(ClientDeleteCollection)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *objectClient[T]) Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error) {
	defer c.recorder.Record(ClientGet)
	return c.inner.Get(ctx, name, opts)
}

func (c *objectClient[T]) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record(ClientUpdate)
	return c.inner.Watch(ctx, opts)
}

func (c *objectClient[T]) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
	defer c.recorder.Record(ClientPatch)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}

type listClient[T any] struct {
	recorder Recorder
	inner    controllerutils.ListClient[T]
}

func (c *listClient[T]) List(ctx context.Context, opts metav1.ListOptions) (T, error) {
	defer c.recorder.Record(ClientList)
	return c.inner.List(ctx, opts)
}

type statusClient[T metav1.Object] struct {
	recorder Recorder
	inner    controllerutils.StatusClient[T]
}

func (c *statusClient[T]) UpdateStatus(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	defer c.recorder.Record(ClientUpdateStatus)
	return c.inner.UpdateStatus(ctx, obj, opts)
}

func ObjectClient[T metav1.Object](recorder Recorder, inner controllerutils.ObjectClient[T],
) controllerutils.ObjectClient[T] {
	return &objectClient[T]{
		recorder: recorder,
		inner:    inner,
	}
}

func StatusClient[T metav1.Object](recorder Recorder, inner controllerutils.StatusClient[T]) controllerutils.StatusClient[T] {
	return &statusClient[T]{
		recorder: recorder,
		inner:    inner,
	}
}

func ListClient[T any](recorder Recorder, inner controllerutils.ListClient[T]) controllerutils.ListClient[T] {
	return &listClient[T]{
		recorder: recorder,
		inner:    inner,
	}
}
