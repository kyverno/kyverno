package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newAdmissionReportV1Alpha1(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterAdmissionReport{Spec: kyvernov2.AdmissionReportSpec{}}
	} else {
		report = &kyvernov1alpha2.AdmissionReport{Spec: kyvernov2.AdmissionReportSpec{}}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	reportutils.SetResourceUid(report, resource.GetUID())
	reportutils.SetResourceGVR(report, gvr)
	reportutils.SetResourceNamespaceAndName(report, resource.GetNamespace(), resource.GetName())
	reportutils.SetManagedByKyvernoLabel(report)
	return report
}

func buildAdmissionReportV1Alpha1(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface {
	report := newAdmissionReportV1Alpha1(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), resource)
	reportutils.SetResponses(report, responses...)
	return report
}

func newAdmissionReportReportV1(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &reportsv1.ClusterEphemeralReport{Spec: reportsv1.EphemeralReportSpec{}}
	} else {
		report = &reportsv1.EphemeralReport{Spec: reportsv1.EphemeralReportSpec{}}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	reportutils.SetResourceUid(report, resource.GetUID())
	reportutils.SetResourceGVR(report, gvr)
	reportutils.SetResourceNamespaceAndName(report, resource.GetNamespace(), resource.GetName())
	reportutils.SetManagedByKyvernoLabel(report)
	return report
}

func buildAdmissionReportReportV1(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface {
	report := newAdmissionReportReportV1(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), resource)
	reportutils.SetResponses(report, responses...)
	return report
}

func newBackgroundScanReportV1Alpha1(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterBackgroundScanReport{}
	} else {
		report = &kyvernov1alpha2.BackgroundScanReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	reportutils.SetResourceUid(report, uid)
	reportutils.SetManagedByKyvernoLabel(report)
	return report
}

func newBackgroundScanReportReportsV1(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	var report kyvernov1alpha2.ReportInterface
	if namespace == "" {
		report = &reportsv1.ClusterEphemeralReport{}
	} else {
		report = &reportsv1.EphemeralReport{}
	}
	report.SetName(name)
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	reportutils.SetResourceUid(report, uid)
	reportutils.SetManagedByKyvernoLabel(report)
	return report
}
