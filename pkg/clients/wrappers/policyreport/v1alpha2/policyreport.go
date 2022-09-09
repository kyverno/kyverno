package v1alpha2

import (
	"context"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type policyReports struct {
	inner             v1alpha2.PolicyReportInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapPolicyReports(c v1alpha2.PolicyReportInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.PolicyReportInterface {
	return &policyReports{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *policyReports) Create(ctx context.Context, o *policyreportv1alpha2.PolicyReport, opts metav1.CreateOptions) (*policyreportv1alpha2.PolicyReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "PolicyReport", c.ns, o, opts, c.inner.Create)
}

func (c *policyReports) Update(ctx context.Context, o *policyreportv1alpha2.PolicyReport, opts metav1.UpdateOptions) (*policyreportv1alpha2.PolicyReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "PolicyReport", c.ns, o, opts, c.inner.Update)
}

func (c *policyReports) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "PolicyReport", c.ns, name, opts, c.inner.Delete)
}

func (c *policyReports) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "PolicyReport", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *policyReports) Get(ctx context.Context, name string, opts metav1.GetOptions) (*policyreportv1alpha2.PolicyReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "PolicyReport", c.ns, name, opts, c.inner.Get)
}

func (c *policyReports) List(ctx context.Context, opts metav1.ListOptions) (*policyreportv1alpha2.PolicyReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "PolicyReport", c.ns, opts, c.inner.List)
}

func (c *policyReports) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "PolicyReport", c.ns, opts, c.inner.Watch)
}

func (c *policyReports) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*policyreportv1alpha2.PolicyReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "PolicyReport", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
