package v1alpha2

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	kyvernov1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type ClusterReportChangeRequestsGetter interface {
	ClusterReportChangeRequests() ClusterReportChangeRequestControlInterface
}

type ClusterReportChangeRequestControlInterface interface {
	Create(ctx context.Context, clusterReportChangeRequest *v1alpha2.ClusterReportChangeRequest, opts metav1.CreateOptions) (*v1alpha2.ClusterReportChangeRequest, error)
	Update(ctx context.Context, clusterReportChangeRequest *v1alpha2.ClusterReportChangeRequest, opts metav1.UpdateOptions) (*v1alpha2.ClusterReportChangeRequest, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ClusterReportChangeRequest, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ClusterReportChangeRequestList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterReportChangeRequest, err error)
}

type clusterReportChangeRequestControl struct {
	client            rest.Interface
	crcrClient        kyvernov1alpha2.ClusterReportChangeRequestsGetter
	clientQueryMetric utils.ClientQueryMetric
}

func newClusterReportChangeRequests(c *KyvernoV1alpha2Client) *clusterReportChangeRequestControl {
	return &clusterReportChangeRequestControl{
		client:            c.RESTClient(),
		crcrClient:        c.kyvernov1alpha2Interface,
		clientQueryMetric: c.clientQueryMetric,
	}
}

func (c *clusterReportChangeRequestControl) Create(ctx context.Context, clusterReportChangeRequest *v1alpha2.ClusterReportChangeRequest, opts metav1.CreateOptions) (*v1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Create(ctx, clusterReportChangeRequest, opts)
}

func (c *clusterReportChangeRequestControl) Update(ctx context.Context, clusterReportChangeRequest *v1alpha2.ClusterReportChangeRequest, opts metav1.UpdateOptions) (*v1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Update(ctx, clusterReportChangeRequest, opts)
}

func (c *clusterReportChangeRequestControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Delete(ctx, name, opts)
}

func (c *clusterReportChangeRequestControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterReportChangeRequestControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ClusterReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Get(ctx, name, opts)
}

func (c *clusterReportChangeRequestControl) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ClusterReportChangeRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().List(ctx, opts)
}

func (c *clusterReportChangeRequestControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Watch(ctx, opts)
}

func (c *clusterReportChangeRequestControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ClusterReportChangeRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ClusterReportChangeRequest", "")
	return c.crcrClient.ClusterReportChangeRequests().Patch(ctx, name, pt, data, opts, subresources...)
}
