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

type ReportChangeRequestsGetter interface {
	ReportChangeRequests(namespace string) ReportChangeRequestControlInterface
}

type ReportChangeRequestControlInterface interface {
	Create(ctx context.Context, creportChangeRequest *v1alpha2.ReportChangeRequest, opts metav1.CreateOptions) (*v1alpha2.ReportChangeRequest, error)
	Update(ctx context.Context, creportChangeRequest *v1alpha2.ReportChangeRequest, opts metav1.UpdateOptions) (*v1alpha2.ReportChangeRequest, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ReportChangeRequest, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ReportChangeRequestList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ReportChangeRequest, err error)
}

type reportChangeRequestControl struct {
	client            rest.Interface
	rcrClient         kyvernov1alpha2.ReportChangeRequestsGetter
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func newReportChangeRequests(c *KyvernoV1alpha2Client, namespace string) *reportChangeRequestControl {
	return &reportChangeRequestControl{
		client:            c.RESTClient(),
		rcrClient:         c.kyvernov1alpha2Interface,
		clientQueryMetric: c.clientQueryMetric,
		ns:                namespace,
	}
}

func (c *reportChangeRequestControl) Create(ctx context.Context, reportChangeRequest *v1alpha2.ReportChangeRequest, opts metav1.CreateOptions) (*v1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Create(ctx, reportChangeRequest, opts)
}

func (c *reportChangeRequestControl) Update(ctx context.Context, reportChangeRequest *v1alpha2.ReportChangeRequest, opts metav1.UpdateOptions) (*v1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Update(ctx, reportChangeRequest, opts)
}

func (c *reportChangeRequestControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Delete(ctx, name, opts)
}

func (c *reportChangeRequestControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).DeleteCollection(ctx, opts, listOpts)
}

func (c *reportChangeRequestControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.ReportChangeRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Get(ctx, name, opts)
}

func (c *reportChangeRequestControl) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.ReportChangeRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).List(ctx, opts)
}

func (c *reportChangeRequestControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Watch(ctx, opts)
}

func (c *reportChangeRequestControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.ReportChangeRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ReportChangeRequest", c.ns)
	return c.rcrClient.ReportChangeRequests(c.ns).Patch(ctx, name, pt, data, opts, subresources...)
}
