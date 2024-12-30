package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &reportsv1.ClusterEphemeralReport{Spec: reportsv1.EphemeralReportSpec{}}
	} else {
		report = &reportsv1.EphemeralReport{Spec: reportsv1.EphemeralReportSpec{}}
	}
	report.SetGenerateName(name + "-")
	report.SetNamespace(namespace)
	SetResourceUid(report, resource.GetUID())
	SetResourceGVR(report, gvr)
	SetResourceGVK(report, gvk)
	SetResourceNamespaceAndName(report, resource.GetNamespace(), resource.GetName())
	SetManagedByKyvernoLabel(report)
	SetSource(report, "admission")
	return report
}

func BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface {
	report := NewAdmissionReport(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), schema.GroupVersionKind(request.Kind), resource)
	SetResponses(report, responses...)
	return report
}

func NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &reportsv1.ClusterEphemeralReport{}
	} else {
		report = &reportsv1.EphemeralReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	SetResourceUid(report, uid)
	SetResourceGVK(report, gvk)
	SetResourceNamespaceAndName(report, namespace, owner)
	SetManagedByKyvernoLabel(report)
	SetSource(report, "background-scan")
	return report
}

func NewPolicyReport(namespace, name string, scope *corev1.ObjectReference, results ...policyreportv1alpha2.PolicyReportResult) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
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
