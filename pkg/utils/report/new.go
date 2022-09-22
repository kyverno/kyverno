package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func NewAdmissionReport(resource metav1.Object, request *admissionv1.AdmissionRequest, gvk metav1.GroupVersionKind, responses ...*response.EngineResponse) kyvernov1alpha2.ReportChangeRequestInterface {
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
	SetResourceGvkLabels(report, gvk.Group, gvk.Version, gvk.Kind)
	SetResponses(report, responses...)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewBackgroundScanReport(resource metav1.Object, gvk schema.GroupVersionKind) kyvernov1alpha2.ReportChangeRequestInterface {
	name := string(resource.GetUID())
	namespace := resource.GetNamespace()
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterBackgroundScanReport{
			Owner: metav1.OwnerReference{
				APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
				Kind:       gvk.Kind,
				Name:       resource.GetName(),
				UID:        resource.GetUID(),
			},
		}
	} else {
		report = &kyvernov1alpha2.BackgroundScanReport{
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
	SetOwner(report, gvk.Group, gvk.Version, gvk.Kind, resource)
	SetResourceLabels(report, resource)
	SetResourceGvkLabels(report, gvk.Group, gvk.Version, gvk.Kind)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewPolicyReport(namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) kyvernov1alpha2.ReportChangeRequestInterface {
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &policyreportv1alpha2.ClusterPolicyReport{
			// Owner: metav1.OwnerReference{
			// 	APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
			// 	Kind:       gvk.Kind,
			// 	Name:       resource.GetName(),
			// 	UID:        resource.GetUID(),
			// },
		}
	} else {
		report = &policyreportv1alpha2.PolicyReport{
			// Owner: metav1.OwnerReference{
			// 	APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
			// 	Kind:       gvk.Kind,
			// 	Name:       resource.GetName(),
			// 	UID:        resource.GetUID(),
			// },
		}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetManagedByKyvernoLabel(report)
	SetResults(report, results...)
	return report
}
