package v1alpha2

import (
	"context"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
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

func (c *clusterPolicyReports) Create(ctx context.Context, o *policyreportv1alpha2.ClusterPolicyReport, opts metav1.CreateOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", o, opts, c.inner.Create)
}

func (c *clusterPolicyReports) Update(ctx context.Context, o *policyreportv1alpha2.ClusterPolicyReport, opts metav1.UpdateOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", o, opts, c.inner.Update)
}

func (c *clusterPolicyReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", name, opts, c.inner.Delete)
}

func (c *clusterPolicyReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", opts, listOpts, c.inner.DeleteCollection)
}

func (c *clusterPolicyReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", name, opts, c.inner.Get)
}

func (c *clusterPolicyReports) List(ctx context.Context, opts metav1.ListOptions) (*policyreportv1alpha2.ClusterPolicyReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", opts, c.inner.List)
}

func (c *clusterPolicyReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", opts, c.inner.Watch)
}

func (c *clusterPolicyReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*policyreportv1alpha2.ClusterPolicyReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ClusterPolicyReport", "", name, pt, data, opts, c.inner.Patch, subresources...)
}
