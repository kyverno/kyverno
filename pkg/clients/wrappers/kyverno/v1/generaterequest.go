package v1

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type generateRequest struct {
	inner             v1.GenerateRequestInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapGenerateRequests(c v1.GenerateRequestInterface, m utils.ClientQueryMetric, namespace string) v1.GenerateRequestInterface {
	return &generateRequest{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *generateRequest) Create(ctx context.Context, o *kyvernov1.GenerateRequest, opts metav1.CreateOptions) (*kyvernov1.GenerateRequest, error) {
	return utils.Create(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, o, opts, c.inner.Create)
}

func (c *generateRequest) Update(ctx context.Context, o *kyvernov1.GenerateRequest, opts metav1.UpdateOptions) (*kyvernov1.GenerateRequest, error) {
	return utils.Update(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, o, opts, c.inner.Update)
}

func (c *generateRequest) UpdateStatus(ctx context.Context, o *kyvernov1.GenerateRequest, opts metav1.UpdateOptions) (*kyvernov1.GenerateRequest, error) {
	return utils.UpdateStatus(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, o, opts, c.inner.UpdateStatus)
}

func (c *generateRequest) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, name, opts, c.inner.Delete)
}

func (c *generateRequest) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *generateRequest) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1.GenerateRequest, error) {
	return utils.Get(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, name, opts, c.inner.Get)
}

func (c *generateRequest) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1.GenerateRequestList, error) {
	return utils.List(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, opts, c.inner.List)
}

func (c *generateRequest) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, opts, c.inner.Watch)
}

func (c *generateRequest) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1.GenerateRequest, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "GenerateRequest", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
