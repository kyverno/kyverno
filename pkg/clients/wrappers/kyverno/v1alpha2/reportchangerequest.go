package v1alpha2

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type reportChangeRequestControl struct {
	inner             v1alpha2.ReportChangeRequestInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapReportChangeRequests(c v1alpha2.ReportChangeRequestInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.ReportChangeRequestInterface {
	return &reportChangeRequestControl{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *reportChangeRequestControl) Create(ctx context.Context, reportChangeRequest *kyvernov1alpha2.ReportChangeRequest, opts metav1.CreateOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Create(ctx, reportChangeRequest, opts)
}

func (c *reportChangeRequestControl) Update(ctx context.Context, reportChangeRequest *kyvernov1alpha2.ReportChangeRequest, opts metav1.UpdateOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Update(ctx, reportChangeRequest, opts)
}

func (c *reportChangeRequestControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *reportChangeRequestControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *reportChangeRequestControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *reportChangeRequestControl) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ReportChangeRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.List(ctx, opts)
}

func (c *reportChangeRequestControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *reportChangeRequestControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov1alpha2.ReportChangeRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
