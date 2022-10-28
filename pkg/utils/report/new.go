package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func NewAdmissionReport(resource unstructured.Unstructured, request *admissionv1.AdmissionRequest, gvk metav1.GroupVersionKind, responses ...*response.EngineResponse) kyvernov1alpha2.ReportInterface {
	name := string(request.UID)
	namespace := resource.GetNamespace()
	owner := resource.GetName()
	uid := resource.GetUID()
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterAdmissionReport{
			Spec: kyvernov1alpha2.AdmissionReportSpec{
				Owner: metav1.OwnerReference{
					APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
					Kind:       gvk.Kind,
					Name:       owner,
					UID:        uid,
				},
			},
		}
	} else {
		report = &kyvernov1alpha2.AdmissionReport{
			Spec: kyvernov1alpha2.AdmissionReportSpec{
				Owner: metav1.OwnerReference{
					APIVersion: metav1.GroupVersion{Group: gvk.Group, Version: gvk.Version}.String(),
					Kind:       gvk.Kind,
					Name:       owner,
					UID:        uid,
				},
			},
		}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetResourceLabels(report, uid)
	SetResourceVersionLabels(report, &resource)
	SetResponses(report, responses...)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterBackgroundScanReport{}
	} else {
		report = &kyvernov1alpha2.BackgroundScanReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	SetResourceLabels(report, uid)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewPolicyReport(namespace, name string, results ...policyreportv1alpha2.PolicyReportResult) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &policyreportv1alpha2.ClusterPolicyReport{}
	} else {
		report = &policyreportv1alpha2.PolicyReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetManagedByKyvernoLabel(report)
	SetResults(report, results...)
	return report
}
