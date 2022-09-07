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

type clusterPolicies struct {
	inner             v1.ClusterPolicyInterface
	clientQueryMetric utils.ClientQueryMetric
}

func wrapClusterPolicies(c v1.ClusterPolicyInterface, m utils.ClientQueryMetric) v1.ClusterPolicyInterface {
	return &clusterPolicies{
		inner:             c,
		clientQueryMetric: m,
	}
}

func (c *clusterPolicies) Create(ctx context.Context, clusterPolicy *kyvernov1.ClusterPolicy, opts metav1.CreateOptions) (*kyvernov1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Create(ctx, clusterPolicy, opts)
}

func (c *clusterPolicies) Update(ctx context.Context, clusterPolicy *kyvernov1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Update(ctx, clusterPolicy, opts)
}

func (c *clusterPolicies) UpdateStatus(ctx context.Context, clusterPolicy *kyvernov1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.UpdateStatus(ctx, clusterPolicy, opts)
}

func (c *clusterPolicies) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Delete(ctx, name, opts)
}

func (c *clusterPolicies) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterPolicies) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Get(ctx, name, opts)
}

func (c *clusterPolicies) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1.ClusterPolicyList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.List(ctx, opts)
}

func (c *clusterPolicies) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Watch(ctx, opts)
}

func (c *clusterPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov1.ClusterPolicy, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.inner.Patch(ctx, name, pt, data, opts, subresources...)
}
