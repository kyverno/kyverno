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

func (c *clusterReportChangeRequest) Create(ctx context.Context, o *kyvernov1alpha2.ClusterReportChangeRequest, opts metav1.CreateOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", o, opts, c.inner.Create)
}

func (c *clusterReportChangeRequest) Update(ctx context.Context, o *kyvernov1alpha2.ClusterReportChangeRequest, opts metav1.UpdateOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", o, opts, c.inner.Update)
}

func (c *clusterReportChangeRequest) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", name, opts, c.inner.Delete)
}

func (c *clusterReportChangeRequest) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", opts, listOpts, c.inner.DeleteCollection)
}

func (c *clusterReportChangeRequest) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", name, opts, c.inner.Get)
}

func (c *clusterReportChangeRequest) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ClusterReportChangeRequestList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", opts, c.inner.List)
}

func (c *clusterReportChangeRequest) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", opts, c.inner.Watch)
}

func (c *clusterReportChangeRequest) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.ClusterReportChangeRequest, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ClusterReportChangeRequest", "", name, pt, data, opts, c.inner.Patch, subresources...)
}
