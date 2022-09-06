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

type clusterReportChangeRequest struct {
	inner             v1alpha2.ClusterReportChangeRequestInterface
	clientQueryMetric utils.ClientQueryMetric
}

func wrapClusterReportChangeRequests(c v1alpha2.ClusterReportChangeRequestInterface, m utils.ClientQueryMetric) v1alpha2.ClusterReportChangeRequestInterface {
	return &clusterReportChangeRequest{
		inner:             c,
		clientQueryMetric: m,
	}
}

func (c *clusterReportChangeRequest) Create(ctx context.Context, clusterReportChangeRequest *kyvernov1alpha2.ClusterReportChangeRequest, opts metav1.CreateOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Create(ctx, clusterReportChangeRequest, opts)
}

func (c *clusterReportChangeRequest) Update(ctx context.Context, clusterReportChangeRequest *kyvernov1alpha2.ClusterReportChangeRequest, opts metav1.UpdateOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Update(ctx, clusterReportChangeRequest, opts)
}

func (c *clusterReportChangeRequest) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Delete(ctx, name, opts)
}

func (c *clusterReportChangeRequest) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterReportChangeRequest) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Get(ctx, name, opts)
}

func (c *clusterReportChangeRequest) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ClusterReportChangeRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.List(ctx, opts)
}

func (c *clusterReportChangeRequest) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Watch(ctx, opts)
}

func (c *clusterReportChangeRequest) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov1alpha2.ClusterReportChangeRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
