package v1beta1

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernov1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type UpdateRequestsGetter interface {
	UpdateRequests(namespace string) UpdateRequestControlInterface
}

type UpdateRequestControlInterface interface {
	Create(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.CreateOptions) (*v1beta1.UpdateRequest, error)
	Update(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.UpdateOptions) (*v1beta1.UpdateRequest, error)
	UpdateStatus(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.UpdateOptions) (*v1beta1.UpdateRequest, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1beta1.UpdateRequest, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1beta1.UpdateRequestList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1beta1.UpdateRequest, err error)
}

type updateRequestsControl struct {
	client            rest.Interface
	urClient          kyvernov1beta1.UpdateRequestsGetter
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func newUpdateRequests(c *KyvernoV1beta1Client, namespace string) *updateRequestsControl {
	return &updateRequestsControl{
		client:            c.RESTClient(),
		urClient:          c.kyvernov1beta1Interface,
		clientQueryMetric: c.clientQueryMetric,
		ns:                namespace,
	}
}

func (c *updateRequestsControl) Create(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.CreateOptions) (*v1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Create(ctx, updateRequest, opts)
}

func (c *updateRequestsControl) Update(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.UpdateOptions) (*v1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Update(ctx, updateRequest, opts)
}

func (c *updateRequestsControl) UpdateStatus(ctx context.Context, updateRequest *v1beta1.UpdateRequest, opts metav1.UpdateOptions) (*v1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).UpdateStatus(ctx, updateRequest, opts)
}

func (c *updateRequestsControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Delete(ctx, name, opts)
}

func (c *updateRequestsControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).DeleteCollection(ctx, opts, listOpts)
}

func (c *updateRequestsControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1beta1.UpdateRequest, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Get(ctx, name, opts)
}

func (c *updateRequestsControl) List(ctx context.Context, opts metav1.ListOptions) (*v1beta1.UpdateRequestList, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).List(ctx, opts)
}

func (c *updateRequestsControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Watch(ctx, opts)
}

func (c *updateRequestsControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1beta1.UpdateRequest, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "UpdateRequest", c.ns)
	return c.urClient.UpdateRequests(c.ns).Patch(ctx, name, pt, data, opts, subresources...)
}
