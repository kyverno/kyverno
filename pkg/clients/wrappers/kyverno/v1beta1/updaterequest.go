package v1beta1

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
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

func (c *updateRequests) Create(ctx context.Context, o *kyvernov1beta1.UpdateRequest, opts metav1.CreateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	return utils.Create(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, o, opts, c.inner.Create)
}

func (c *updateRequests) Update(ctx context.Context, o *kyvernov1beta1.UpdateRequest, opts metav1.UpdateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	return utils.Update(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, o, opts, c.inner.Update)
}

func (c *updateRequests) UpdateStatus(ctx context.Context, o *kyvernov1beta1.UpdateRequest, opts metav1.UpdateOptions) (*kyvernov1beta1.UpdateRequest, error) {
	return utils.UpdateStatus(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, o, opts, c.inner.UpdateStatus)
}

func (c *updateRequests) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, name, opts, c.inner.Delete)
}

func (c *updateRequests) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *updateRequests) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1beta1.UpdateRequest, error) {
	return utils.Get(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, name, opts, c.inner.Get)
}

func (c *updateRequests) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1beta1.UpdateRequestList, error) {
	return utils.List(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, opts, c.inner.List)
}

func (c *updateRequests) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, opts, c.inner.Watch)
}

func (c *updateRequests) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1beta1.UpdateRequest, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "UpdateRequest", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
