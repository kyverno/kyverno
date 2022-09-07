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

type clusterPolicyReports struct {
	inner             v1alpha2.ClusterPolicyReportInterface
	clientQueryMetric utils.ClientQueryMetric
}

func wrapClusterPolicyReports(c v1alpha2.ClusterPolicyReportInterface, m utils.ClientQueryMetric) v1alpha2.ClusterPolicyReportInterface {
	return &clusterPolicyReports{
		inner:             c,
		clientQueryMetric: m,
	}
}

func (c *clusterPolicyReports) Create(ctx context.Context, clusterPolicyReport *policyreportv1alpha2.ClusterPolicyReport, opts metav1.CreateOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Create(ctx, clusterPolicyReport, opts)
}

func (c *clusterPolicyReports) Update(ctx context.Context, clusterPolicyReport *policyreportv1alpha2.ClusterPolicyReport, opts metav1.UpdateOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Update(ctx, clusterPolicyReport, opts)
}

func (c *clusterPolicyReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Delete(ctx, name, opts)
}

func (c *clusterPolicyReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterPolicyReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Get(ctx, name, opts)
}

func (c *clusterPolicyReports) List(ctx context.Context, opts metav1.ListOptions) (*policyreportv1alpha2.ClusterPolicyReportList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.List(ctx, opts)
}

func (c *clusterPolicyReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Watch(ctx, opts)
}

func (c *clusterPolicyReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *policyreportv1alpha2.ClusterPolicyReport, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
