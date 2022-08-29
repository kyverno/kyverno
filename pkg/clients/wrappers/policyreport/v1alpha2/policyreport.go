package v1alpha2

import (
	"context"

	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	"github.com/kyverno/kyverno/pkg/metrics"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

type PolicyReportsGetter interface {
	PolicyReports(namespace string) PolicyReportControlInterface
}

type PolicyReportControlInterface interface {
	Create(ctx context.Context, policyReport *v1alpha2.PolicyReport, opts metav1.CreateOptions) (*v1alpha2.PolicyReport, error)
	Update(ctx context.Context, policyReport *v1alpha2.PolicyReport, opts metav1.UpdateOptions) (*v1alpha2.PolicyReport, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.PolicyReport, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.PolicyReportList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.PolicyReport, err error)
}

type policyReportsControl struct {
	client            rest.Interface
	polrClient        policyreportv1alpha2.PolicyReportsGetter
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

// newPolicyReports returns a PolicyReports
func newPolicyReports(c *Wgpolicyk8sV1alpha2Client, namespace string) *policyReportsControl {
	return &policyReportsControl{
		client:            c.RESTClient(),
		polrClient:        c.wgpolicyk8sV1alpha2Interface,
		clientQueryMetric: c.clientQueryMetric,
		ns:                namespace,
	}
}

func (c *policyReportsControl) Create(ctx context.Context, policyReport *v1alpha2.PolicyReport, opts metav1.CreateOptions) (*v1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientCreate, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Create(ctx, policyReport, opts)
}

func (c *policyReportsControl) Update(ctx context.Context, policyReport *v1alpha2.PolicyReport, opts metav1.UpdateOptions) (*v1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientUpdate, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Update(ctx, policyReport, opts)
}

func (c *policyReportsControl) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDelete, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Delete(ctx, name, opts)
}

func (c *policyReportsControl) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	c.clientQueryMetric.Record(metrics.ClientDeleteCollection, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).DeleteCollection(ctx, opts, listOpts)
}

func (c *policyReportsControl) Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1alpha2.PolicyReport, error) {
	c.clientQueryMetric.Record(metrics.ClientGet, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Get(ctx, name, opts)
}

func (c *policyReportsControl) List(ctx context.Context, opts metav1.ListOptions) (*v1alpha2.PolicyReportList, error) {
	c.clientQueryMetric.Record(metrics.ClientList, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).List(ctx, opts)
}

func (c *policyReportsControl) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	c.clientQueryMetric.Record(metrics.ClientWatch, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Watch(ctx, opts)
}

func (c *policyReportsControl) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1alpha2.PolicyReport, err error) {
	c.clientQueryMetric.Record(metrics.ClientPatch, metrics.PolicyReportClient, "PolicyReport", c.ns)
	return c.polrClient.PolicyReports(c.ns).Patch(ctx, name, pt, data, opts, subresources...)
}
