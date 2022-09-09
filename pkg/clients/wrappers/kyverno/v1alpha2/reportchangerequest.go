package v1alpha2

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type reportChangeRequest struct {
	inner             v1alpha2.ReportChangeRequestInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapReportChangeRequests(c v1alpha2.ReportChangeRequestInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.ReportChangeRequestInterface {
	return &reportChangeRequest{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *reportChangeRequest) Create(ctx context.Context, o *kyvernov1alpha2.ReportChangeRequest, opts metav1.CreateOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, o, opts, c.inner.Create)
}

func (c *reportChangeRequest) Update(ctx context.Context, o *kyvernov1alpha2.ReportChangeRequest, opts metav1.UpdateOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, o, opts, c.inner.Update)
}

func (c *reportChangeRequest) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, name, opts, c.inner.Delete)
}

func (c *reportChangeRequest) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *reportChangeRequest) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, name, opts, c.inner.Get)
}

func (c *reportChangeRequest) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ReportChangeRequestList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, opts, c.inner.List)
}

func (c *reportChangeRequest) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, opts, c.inner.Watch)
}

func (c *reportChangeRequest) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.ReportChangeRequest, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ReportChangeRequest", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
