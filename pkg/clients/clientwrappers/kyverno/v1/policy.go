package v1

import (
	"context"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/clientwrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type PoliciesGetter interface {
	Policies(namespace string) PoliciesControlInterface
}

type PoliciesControlInterface interface {
	Create(ctx context.Context, policy *v1.Policy, opts metav1.CreateOptions) (*v1.Policy, error)
	Update(ctx context.Context, policy *v1.Policy, opts metav1.UpdateOptions) (*v1.Policy, error)
	UpdateStatus(ctx context.Context, policy *v1.Policy, opts metav1.UpdateOptions) (*v1.Policy, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Policy, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.PolicyList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Policy, err error)
}

type policiesControl struct {
	client            rest.Interface
	polClient         kyvernov1.PoliciesGetter
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func newPolicies(c *KyvernoV1Client, namespace string) *policiesControl {
	return &policiesControl{
		client:            c.RESTClient(),
		polClient:         c.kyvernov1Interface,
		clientQueryMetric: c.clientQueryMetric,
		ns:                namespace,
	}
}

func (c *policiesControl) Create(ctx context.Context, policy *v1.Policy, opts metav1.CreateOptions) (*v1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Create(ctx, policy, opts)
}

func (c *policiesControl) Update(ctx context.Context, policy *v1.Policy, opts metav1.UpdateOptions) (*v1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Update(ctx, policy, opts)
}

func (c *policiesControl) UpdateStatus(ctx context.Context, policy *v1.Policy, opts metav1.UpdateOptions) (*v1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).UpdateStatus(ctx, policy, opts)
}

func (c *policiesControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Delete(ctx, name, opts)
}

func (c *policiesControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).DeleteCollection(ctx, opts, listOpts)
}

func (c *policiesControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Get(ctx, name, opts)
}

func (c *policiesControl) List(ctx context.Context, opts metav1.ListOptions) (*v1.PolicyList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).List(ctx, opts)
}

func (c *policiesControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Watch(ctx, opts)
}

func (c *policiesControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.Policy, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "Policy", c.ns)
	return c.polClient.Policies(c.ns).Patch(ctx, name, pt, data, opts, subresources...)
}
