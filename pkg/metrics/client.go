package metrics

import (
	"context"

	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type Recorder interface {
	Record(clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string)
}

type clientQueryRecorder struct {
	manager MetricsConfigManager
}

func NewClientQueryRecorder(m MetricsConfigManager) Recorder {
	return &clientQueryRecorder{
		manager: m,
	}
}

func (r *clientQueryRecorder) Record(clientQueryOperation ClientQueryOperation, clientType ClientType, resourceKind string, resourceNamespace string) {
	r.manager.RecordClientQueries(clientQueryOperation, clientType, resourceKind, resourceNamespace)
}

type client[T metav1.Object, L any] struct {
	recorder   Recorder
	ns         string
	kind       string
	clientType ClientType
	inner      controllerutils.Client[T, L]
}

func (c *client[T, L]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	defer c.recorder.Record(ClientCreate, c.clientType, c.kind, c.ns)
	return c.inner.Create(ctx, obj, opts)
}

func (c *client[T, L]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	defer c.recorder.Record(ClientUpdate, c.clientType, c.kind, c.ns)
	return c.inner.Update(ctx, obj, opts)
}

func (c *client[T, L]) List(ctx context.Context, opts metav1.ListOptions) (L, error) {
	defer c.recorder.Record(ClientList, c.clientType, c.kind, c.ns)
	return c.inner.List(ctx, opts)
}

func (c *client[T, L]) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	defer c.recorder.Record(ClientDelete, c.clientType, c.kind, c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *client[T, L]) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	defer c.recorder.Record(ClientDeleteCollection, c.clientType, c.kind, c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *client[T, L]) Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error) {
	defer c.recorder.Record(ClientGet, c.clientType, c.kind, c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *client[T, L]) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record(ClientUpdate, c.clientType, c.kind, c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *client[T, L]) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error) {
	defer c.recorder.Record(ClientPatch, c.clientType, c.kind, c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}

type statusClient[T metav1.Object] struct {
	recorder   Recorder
	ns         string
	kind       string
	clientType ClientType
	inner      controllerutils.StatusClient[T]
}

func (c *statusClient[T]) UpdateStatus(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	defer c.recorder.Record(ClientUpdateStatus, c.clientType, c.kind, c.ns)
	return c.inner.UpdateStatus(ctx, obj, opts)
}

func NamespacedClient[T metav1.Object, L any](
	recorder Recorder,
	ns string,
	kind string,
	clientType ClientType,
	inner controllerutils.Client[T, L],
) controllerutils.Client[T, L] {
	return &client[T, L]{
		recorder:   recorder,
		ns:         ns,
		kind:       kind,
		clientType: clientType,
		inner:      inner,
	}
}

func NamespacedStatusClient[T metav1.Object](recorder Recorder, ns string, kind string, clientType ClientType, inner controllerutils.StatusClient[T]) controllerutils.StatusClient[T] {
	return &statusClient[T]{
		recorder:   recorder,
		ns:         ns,
		kind:       kind,
		clientType: clientType,
		inner:      inner,
	}
}

func ClusteredClient[T metav1.Object, L any](
	recorder Recorder,
	kind string,
	clientType ClientType,
	inner controllerutils.Client[T, L],
) controllerutils.Client[T, L] {
	return &client[T, L]{
		recorder:   recorder,
		kind:       kind,
		clientType: clientType,
		inner:      inner,
	}
}
