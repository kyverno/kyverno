package v2beta1

import (
	"context"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	versionedkyvernov2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type ClusterPoliciesGetter interface {
	ClusterPolicies() ClusterPoliciesControlInterface
}

type ClusterPoliciesControlInterface interface {
	Create(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.CreateOptions) (*kyvernov2beta1.ClusterPolicy, error)
	Update(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov2beta1.ClusterPolicy, error)
	UpdateStatus(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov2beta1.ClusterPolicy, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov2beta1.ClusterPolicy, error)
	List(ctx context.Context, opts metav1.ListOptions) (*kyvernov2beta1.ClusterPolicyList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov2beta1.ClusterPolicy, err error)
}

type clusterPoliciesControl struct {
	client            rest.Interface
	cpolClient        versionedkyvernov2beta1.ClusterPoliciesGetter
	clientQueryMetric utils.ClientQueryMetric
}

func newClusterPolicies(c *KyvernoV2beta1Client) *clusterPoliciesControl {
	return &clusterPoliciesControl{
		client:            c.RESTClient(),
		cpolClient:        c.kyvernov2beta1Interface,
		clientQueryMetric: c.clientQueryMetric,
	}
}

func (c *clusterPoliciesControl) Create(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.CreateOptions) (*kyvernov2beta1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Create(ctx, clusterPolicy, opts)
}

func (c *clusterPoliciesControl) Update(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov2beta1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Update(ctx, clusterPolicy, opts)
}

func (c *clusterPoliciesControl) UpdateStatus(ctx context.Context, clusterPolicy *kyvernov2beta1.ClusterPolicy, opts metav1.UpdateOptions) (*kyvernov2beta1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdateStatus, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().UpdateStatus(ctx, clusterPolicy, opts)
}

func (c *clusterPoliciesControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Delete(ctx, name, opts)
}

func (c *clusterPoliciesControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().DeleteCollection(ctx, opts, listOpts)
}

func (c *clusterPoliciesControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov2beta1.ClusterPolicy, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Get(ctx, name, opts)
}

func (c *clusterPoliciesControl) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov2beta1.ClusterPolicyList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().List(ctx, opts)
}

func (c *clusterPoliciesControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Watch(ctx, opts)
}

func (c *clusterPoliciesControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *kyvernov2beta1.ClusterPolicy, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.KyvernoClient, "ClusterPolicy", "")
	return c.cpolClient.ClusterPolicies().Patch(ctx, name, pt, data, opts, subresources...)
}
