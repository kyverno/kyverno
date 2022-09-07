package v1alpha2

import (
	"context"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type policyReports struct {
	inner             v1alpha2.PolicyReportInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapPolicyReports(c v1alpha2.PolicyReportInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.PolicyReportInterface {
	return &policyReports{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *policyReports) Create(ctx context.Context, policyReport *policyreportv1alpha2.PolicyReport, opts metav1.CreateOptions) (*policyreportv1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Create(ctx, policyReport, opts)
}

func (c *policyReports) Update(ctx context.Context, policyReport *policyreportv1alpha2.PolicyReport, opts metav1.UpdateOptions) (*policyreportv1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Update(ctx, policyReport, opts)
}

func (c *policyReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *policyReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *policyReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*policyreportv1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *policyReports) List(ctx context.Context, opts metav1.ListOptions) (*policyreportv1alpha2.PolicyReportList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.List(ctx, opts)
}

func (c *policyReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *policyReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *policyreportv1alpha2.PolicyReport, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
