package v1alpha2

import (
	"context"

	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type ClusterPolicyReportsGetter interface {
	ClusterPolicyReports() ClusterPolicyReportControlInterface
}

type ClusterPolicyReportControlInterface interface {
	Create(ctx context.Context, clusterPolicyReport *v1alpha2.ClusterPolicyReport, opts metav1.CreateOptions) (*v1alpha2.ClusterPolicyReport, error)
	Update(ctx context.Context, clusterPolicyReport *v1alpha2.ClusterPolicyReport, opts metav1.UpdateOptions) (*v1alpha2.ClusterPolicyReport, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ClusterPolicyReport, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ClusterPolicyReportList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterPolicyReport, err error)
}

type clusterPolicyReportsControl struct {
	client            rest.Interface
	cpolrClient       policyreportv1alpha2.ClusterPolicyReportsGetter
	clientQueryMetric utils.ClientQueryMetric
}

func newClusterPolicyReports(c *Wgpolicyk8sV1alpha2Client) *clusterPolicyReportsControl {
	return &clusterPolicyReportsControl{
		client:            c.RESTClient(),
		cpolrClient:       c.wgpolicyk8sV1alpha2Interface,
		clientQueryMetric: c.clientQueryMetric,
	}
}

func (c *clusterPolicyReportsControl) Create(ctx context.Context, clusterPolicyReport *v1alpha2.ClusterPolicyReport, opts metav1.CreateOptions) (*v1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Create(ctx, clusterPolicyReport, opts)
}

func (c *clusterPolicyReportsControl) Update(ctx context.Context, clusterPolicyReport *v1alpha2.ClusterPolicyReport, opts metav1.UpdateOptions) (*v1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Update(ctx, clusterPolicyReport, opts)
}

func (c *clusterPolicyReportsControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Delete(ctx, name, opts)
}

func (c *clusterPolicyReportsControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterPolicyReportsControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ClusterPolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Get(ctx, name, opts)
}

func (c *clusterPolicyReportsControl) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ClusterPolicyReportList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().List(ctx, opts)
}

func (c *clusterPolicyReportsControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Watch(ctx, opts)
}

func (c *clusterPolicyReportsControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterPolicyReport, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.PolicyReportClient, "ClusterPolicyReport", "")
	return c.cpolrClient.ClusterPolicyReports().Patch(ctx, name, pt, data, opts, subresources...)
}
