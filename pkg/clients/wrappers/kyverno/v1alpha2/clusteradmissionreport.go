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

type clusterAdmissionReports struct {
	inner             v1alpha2.ClusterAdmissionReportInterface
	clientQueryMetric utils.ClientQueryMetric
}

func wrapClusterAdmissionReports(c v1alpha2.ClusterAdmissionReportInterface, m utils.ClientQueryMetric) v1alpha2.ClusterAdmissionReportInterface {
	return &clusterAdmissionReports{
		inner:             c,
		clientQueryMetric: m,
	}
}

func (c *clusterAdmissionReports) Create(ctx context.Context, o *kyvernov1alpha2.ClusterAdmissionReport, opts metav1.CreateOptions) (*kyvernov1alpha2.ClusterAdmissionReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", o, opts, c.inner.Create)
}

func (c *clusterAdmissionReports) Update(ctx context.Context, o *kyvernov1alpha2.ClusterAdmissionReport, opts metav1.UpdateOptions) (*kyvernov1alpha2.ClusterAdmissionReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", o, opts, c.inner.Update)
}

func (c *clusterAdmissionReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", name, opts, c.inner.Delete)
}

func (c *clusterAdmissionReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", opts, listOpts, c.inner.DeleteCollection)
}

func (c *clusterAdmissionReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.ClusterAdmissionReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", name, opts, c.inner.Get)
}

func (c *clusterAdmissionReports) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.ClusterAdmissionReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", opts, c.inner.List)
}

func (c *clusterAdmissionReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", opts, c.inner.Watch)
}

func (c *clusterAdmissionReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.ClusterAdmissionReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ClusterAdmissionReport", "", name, pt, data, opts, c.inner.Patch, subresources...)
}
