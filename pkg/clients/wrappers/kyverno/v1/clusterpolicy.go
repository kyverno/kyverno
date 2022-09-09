package v1

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
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

func (c *clusterPolicies) Create(ctx context.Context, o *kyvernov1.ClusterPolicy, opts metav1.CreateOptions) (*kyvernov1.ClusterPolicy, error) {
	return utils.Create(ctx, c.clientQueryMetric, "ClusterPolicy", "", o, opts, c.inner.Create)
}

func (c *clusterPolicies) Update(ctx context.Context, o *kyvernov1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov1.ClusterPolicy, error) {
	return utils.Update(ctx, c.clientQueryMetric, "ClusterPolicy", "", o, opts, c.inner.Update)
}

func (c *clusterPolicies) UpdateStatus(ctx context.Context, o *kyvernov1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov1.ClusterPolicy, error) {
	return utils.UpdateStatus(ctx, c.clientQueryMetric, "ClusterPolicy", "", o, opts, c.inner.UpdateStatus)
}

func (c *clusterPolicies) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "ClusterPolicy", "", name, opts, c.inner.Delete)
}

func (c *clusterPolicies) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "ClusterPolicy", "", opts, listOpts, c.inner.DeleteCollection)
}

func (c *clusterPolicies) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1.ClusterPolicy, error) {
	return utils.Get(ctx, c.clientQueryMetric, "ClusterPolicy", "", name, opts, c.inner.Get)
}

func (c *clusterPolicies) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1.ClusterPolicyList, error) {
	return utils.List(ctx, c.clientQueryMetric, "ClusterPolicy", "", opts, c.inner.List)
}

func (c *clusterPolicies) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "ClusterPolicy", "", opts, c.inner.Watch)
}

func (c *clusterPolicies) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1.ClusterPolicy, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "ClusterPolicy", "", name, pt, data, opts, c.inner.Patch, subresources...)
}
