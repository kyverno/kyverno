package v1beta1

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type updateRequests struct {
	inner             v1beta1.UpdateRequestInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapUpdateRequests(c v1beta1.UpdateRequestInterface, m utils.ClientQueryMetric, namespace string) v1beta1.UpdateRequestInterface {
	return &updateRequests{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *updateRequests) Create(ctx context.Context, updateRequest *kyvernov1beta1.UpdateRequest, opts metav1.CreateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Create(ctx, updateRequest, opts)
}

func (c *updateRequests) Update(ctx context.Context, updateRequest *kyvernov1beta1.UpdateRequest, opts metav1.UpdateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Update(ctx, updateRequest, opts)
}

func (c *updateRequests) UpdateStatus(ctx context.Context, updateRequest *kyvernov1beta1.UpdateRequest, opts metav1.UpdateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.UpdateStatus(ctx, updateRequest, opts)
}

func (c *updateRequests) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *updateRequests) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *updateRequests) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *updateRequests) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1beta1.UpdateRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.List(ctx, opts)
}

func (c *updateRequests) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *updateRequests) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov1beta1.UpdateRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
