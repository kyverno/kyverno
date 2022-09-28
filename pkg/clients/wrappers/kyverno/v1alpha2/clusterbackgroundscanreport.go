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

type clusterBackgroundScanReports struct {
	inner             v1alpha2.ClusterBackgroundScanReportInterface
	clientQueryMetric utils.ClientQueryMetric
}

func wrapClusterBackgroundScanReports(c v1alpha2.ClusterBackgroundScanReportInterface, m utils.ClientQueryMetric) v1alpha2.ClusterBackgroundScanReportInterface {
	return &clusterBackgroundScanReports{
		inner:             c,
		clientQueryMetric: m,
	}
}

func (c *clusterBackgroundScanReports) Create(ctx context.Context, o *kyvernov1alpha2.ClusterBackgroundScanReport, opts metav1.CreateOptions) (*kyvernov1alpha2.ClusterBackgroundScanReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", o, opts, c.inner.Create)
}

func (c *clusterBackgroundScanReports) Update(ctx context.Context, o *kyvernov1alpha2.ClusterBackgroundScanReport, opts metav1.UpdateOptions) (*kyvernov1alpha2.ClusterBackgroundScanReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", o, opts, c.inner.Update)
}

func (c *clusterBackgroundScanReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", name, opts, c.inner.Delete)
}

func (c *clusterBackgroundScanReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", opts, listOpts, c.inner.DeleteCollection)
}

func (c *clusterBackgroundScanReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ClusterBackgroundScanReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", name, opts, c.inner.Get)
}

func (c *clusterBackgroundScanReports) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ClusterBackgroundScanReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", opts, c.inner.List)
}

func (c *clusterBackgroundScanReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", opts, c.inner.Watch)
}

func (c *clusterBackgroundScanReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.ClusterBackgroundScanReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ClusterBackgroundScanReport", "", name, pt, data, opts, c.inner.Patch, subresources...)
}
