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

type backgroundScanReport struct {
	inner             v1alpha2.BackgroundScanReportInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapBackgroundScanReports(c v1alpha2.BackgroundScanReportInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.BackgroundScanReportInterface {
	return &backgroundScanReport{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *backgroundScanReport) Create(ctx context.Context, o *kyvernov1alpha2.BackgroundScanReport, opts metav1.CreateOptions) (*kyvernov1alpha2.BackgroundScanReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, o, opts, c.inner.Create)
}

func (c *backgroundScanReport) Update(ctx context.Context, o *kyvernov1alpha2.BackgroundScanReport, opts metav1.UpdateOptions) (*kyvernov1alpha2.BackgroundScanReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, o, opts, c.inner.Update)
}

func (c *backgroundScanReport) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, name, opts, c.inner.Delete)
}

func (c *backgroundScanReport) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *backgroundScanReport) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.BackgroundScanReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, name, opts, c.inner.Get)
}

func (c *backgroundScanReport) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.BackgroundScanReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, opts, c.inner.List)
}

func (c *backgroundScanReport) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, opts, c.inner.Watch)
}

func (c *backgroundScanReport) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.BackgroundScanReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "BackgroundScanReport", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
