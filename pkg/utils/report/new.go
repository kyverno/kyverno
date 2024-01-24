package report

import (
	kyvernoreports "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernoreports.ReportInterface {
	var report kyvernoreports.ReportInterface
	if namespace == "" {
		report = &kyvernoreports.ClusterAdmissionReport{Spec: kyvernoreports.AdmissionReportSpec{}}
	} else {
		report = &kyvernoreports.AdmissionReport{Spec: kyvernoreports.AdmissionReportSpec{}}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetResourceUid(report, resource.GetUID())
	SetResourceGVR(report, gvr)
	SetResourceNamespaceAndName(report, resource.GetNamespace(), resource.GetName())
	SetManagedByKyvernoLabel(report)
	return report
}

func BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernoreports.ReportInterface {
	report := NewAdmissionReport(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), resource)
	SetResponses(report, responses...)
	return report
}

func NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernoreports.ReportInterface {
	var report kyvernoreports.ReportInterface
	if namespace == "" {
		report = &kyvernoreports.ClusterBackgroundScanReport{}
	} else {
		report = &kyvernoreports.BackgroundScanReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	SetResourceUid(report, uid)
	SetManagedByKyvernoLabel(report)
	return report
}

func NewPolicyReport(namespace, name string, scope *corev1.ObjectReference, results ...policyreportv1alpha2.PolicyReportResult) kyvernoreports.ReportInterface {
	var report kyvernoreports.ReportInterface
	if namespace == "" {
		report = &policyreportv1alpha2.ClusterPolicyReport{
			Scope: scope,
		}
	} else {
		report = &policyreportv1alpha2.PolicyReport{
			Scope: scope,
		}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	SetManagedByKyvernoLabel(report)
	SetResults(report, results...)
	return report
}
