package v1

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type policies struct {
	inner             v1.PolicyInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapPolicies(c v1.PolicyInterface, m utils.ClientQueryMetric, namespace string) v1.PolicyInterface {
	return &policies{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *policies) Create(ctx context.Context, policy *kyvernov1.Policy, opts metav1.CreateOptions) (*kyvernov1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Create(ctx, policy, opts)
}

func (c *policies) Update(ctx context.Context, policy *kyvernov1.Policy, opts metav1.UpdateOptions) (*kyvernov1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Update(ctx, policy, opts)
}

func (c *policies) UpdateStatus(ctx context.Context, policy *kyvernov1.Policy, opts metav1.UpdateOptions) (*kyvernov1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.UpdateStatus(ctx, policy, opts)
}

func (c *policies) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Delete(ctx, name, opts)
}

func (c *policies) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *policies) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1.Policy, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Get(ctx, name, opts)
}

func (c *policies) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1.PolicyList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.List(ctx, opts)
}

func (c *policies) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Watch(ctx, opts)
}

func (c *policies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov1.Policy, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "Policy", c.ns)
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
