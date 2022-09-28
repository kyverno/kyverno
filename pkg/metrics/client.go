package metrics

import (
	"context"

	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type client[T metav1.Object] struct {
	metricsConfig MetricsConfigManager
	ns            string
	kind          string
	clientType    ClientType
	inner         controllerutils.Client[T]
}

func (c *client[T]) Create(ctx context.Context, obj T, opts metav1.CreateOptions) (T, error) {
	defer c.metricsConfig.RecordClientQueries(ClientCreate, c.clientType, c.kind, c.ns)
	return c.inner.Create(ctx, obj, opts)
}

func (c *client[T]) Update(ctx context.Context, obj T, opts metav1.UpdateOptions) (T, error) {
	defer c.metricsConfig.RecordClientQueries(ClientUpdate, c.clientType, c.kind, c.ns)
	return c.inner.Update(ctx, obj, opts)
}

func (c *client[T]) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	defer c.metricsConfig.RecordClientQueries(ClientDelete, c.clientType, c.kind, c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *client[T]) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	defer c.metricsConfig.RecordClientQueries(ClientDeleteCollection, c.clientType, c.kind, c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *client[T]) Get(ctx context.Context, name string, opts metav1.GetOptions) (T, error) {
	defer c.metricsConfig.RecordClientQueries(ClientGet, c.clientType, c.kind, c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *client[T]) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	defer c.metricsConfig.RecordClientQueries(ClientUpdate, c.clientType, c.kind, c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *client[T]) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result T, err error) {
	defer c.metricsConfig.RecordClientQueries(ClientPatch, c.clientType, c.kind, c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}

func Client[T metav1.Object](
	metricsConfig MetricsConfigManager,
	ns string,
	kind string,
	clientType ClientType,
	inner controllerutils.Client[T],
) *client[T] {
	return &client[T]{
		metricsConfig: metricsConfig,
		ns:            ns,
		kind:          kind,
		clientType:    clientType,
		inner:         inner,
	}
}
