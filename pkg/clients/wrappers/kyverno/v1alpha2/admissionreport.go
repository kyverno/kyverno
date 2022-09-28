package v1alpha2

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/clients/wrappers/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
)

type admissionReport struct {
	inner             v1alpha2.AdmissionReportInterface
	clientQueryMetric utils.ClientQueryMetric
	ns                string
}

func wrapAdmissionReports(c v1alpha2.AdmissionReportInterface, m utils.ClientQueryMetric, namespace string) v1alpha2.AdmissionReportInterface {
	return &admissionReport{
		inner:             c,
		clientQueryMetric: m,
		ns:                namespace,
	}
}

func (c *admissionReport) Create(ctx context.Context, o *kyvernov1alpha2.AdmissionReport, opts metav1.CreateOptions) (*kyvernov1alpha2.AdmissionReport, error) {
	return utils.Create(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, o, opts, c.inner.Create)
}

func (c *admissionReport) Update(ctx context.Context, o *kyvernov1alpha2.AdmissionReport, opts metav1.UpdateOptions) (*kyvernov1alpha2.AdmissionReport, error) {
	return utils.Update(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, o, opts, c.inner.Update)
}

func (c *admissionReport) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return utils.Delete(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, name, opts, c.inner.Delete)
}

func (c *admissionReport) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	return utils.DeleteCollection(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, opts, listOpts, c.inner.DeleteCollection)
}

func (c *admissionReport) Get(ctx context.Context, name string, opts metav1.GetOptions) (*kyvernov1alpha2.AdmissionReport, error) {
	return utils.Get(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, name, opts, c.inner.Get)
}

func (c *admissionReport) List(ctx context.Context, opts metav1.ListOptions) (*kyvernov1alpha2.AdmissionReportList, error) {
	return utils.List(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, opts, c.inner.List)
}

func (c *admissionReport) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return utils.Watch(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, opts, c.inner.Watch)
}

func (c *admissionReport) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*kyvernov1alpha2.AdmissionReport, error) {
	return utils.Patch(ctx, c.clientQueryMetric, "AdmissionReport", c.ns, name, pt, data, opts, c.inner.Patch, subresources...)
}
