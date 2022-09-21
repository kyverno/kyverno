package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewReport(namespace, name string) kyvernov1alpha2.ReportChangeRequestInterface {
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterReportChangeRequest{}
	} else {
		report = &kyvernov1alpha2.ReportChangeRequest{}
	}
	report.SetName(name)
	report.SetNamespace(name)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewAdmissionReport(
	resource metav1.Object,
	request *admissionv1.AdmissionRequest,
	gvk metav1.GroupVersionKind,
	responses ...*response.EngineResponse,
) kyvernov1alpha2.ReportChangeRequestInterface {
	name := string(request.UID)
	namespace := resource.GetNamespace()
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterAdmissionReport{
			Owner: metav1.OwnerReference{
				APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
				Kind:       gvk.Kind,
				Name:       resource.GetName(),
				UID:        resource.GetUID(),
			},
		}
	} else {
		report = &kyvernov1alpha2.AdmissionReport{
			Owner: metav1.OwnerReference{
				APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
				Kind:       gvk.Kind,
				Name:       resource.GetName(),
				UID:        resource.GetUID(),
			},
		}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetAdmissionLabels(report, request)
	SetResourceLabels(report, resource)
	SetResourceGvkLabels(report, request.Kind.Group, request.Kind.Version, request.Kind.Kind)
	SetResults(report, responses...)
	SetManagedByKyvernoLabel(report)
	return report
}
