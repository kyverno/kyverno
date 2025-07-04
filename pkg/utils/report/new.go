package report

import (
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

func NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, resource unstructured.Unstructured) reportsv1.ReportInterface {
	var report reportsv1.ReportInterface
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

func BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) reportsv1.ReportInterface {
	report := NewAdmissionReport(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), schema.GroupVersionKind(request.Kind), resource)
	SetResponses(report, responses...)
	return report
}

func NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) reportsv1.ReportInterface {
	var report reportsv1.ReportInterface
	if namespace == "" {
		report = &reportsv1.ClusterEphemeralReport{}
	} else {
		report = &reportsv1.EphemeralReport{}
	}
	report.SetGenerateName(name + "-")
	report.SetNamespace(namespace)
	controllerutils.SetOwner(report, gvk.GroupVersion().String(), gvk.Kind, owner, uid)
	SetResourceUid(report, uid)
	SetResourceGVK(report, gvk)
	SetResourceNamespaceAndName(report, namespace, owner)
	SetManagedByKyvernoLabel(report)
	SetSource(report, "background-scan")
	return report
}

func BuildMutationReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) reportsv1.ReportInterface {
	report := NewAdmissionReport(resource.GetNamespace(), string(request.UID), schema.GroupVersionResource(request.Resource), schema.GroupVersionKind(request.Kind), resource)
	SetMutationResponses(report, responses...)
	return report
}

func BuildMutateExistingReport(namespace string, gvk schema.GroupVersionKind, owner string, uid types.UID, responses ...engineapi.EngineResponse) reportsv1.ReportInterface {
	report := NewBackgroundScanReport(namespace, string(uid), gvk, owner, uid)
	SetMutationResponses(report, responses...)
	return report
}

func BuildGenerateReport(namespace string, gvk schema.GroupVersionKind, owner string, uid types.UID, responses ...engineapi.EngineResponse) reportsv1.ReportInterface {
	report := NewBackgroundScanReport(namespace, string(uid), gvk, owner, uid)
	SetGenerationResponses(report, responses...)
	return report
}

func NewPolicyReport(namespace, name string, scope *corev1.ObjectReference, useOpenreports bool, results ...openreportsv1alpha1.ReportResult) reportsv1.ReportInterface {
	var report reportsv1.ReportInterface
	if useOpenreports {
		if namespace == "" {
			report = &openreports.ClusterReportAdapter{
				ClusterReport: &openreportsv1alpha1.ClusterReport{
					Scope: scope,
				},
			}
		} else {
			report = &openreports.ReportAdapter{
				Report: &openreportsv1alpha1.Report{
					Scope: scope,
				},
			}
		}
	} else {
		if namespace == "" {
			report = openreports.NewWGCpolAdapter(&v1alpha2.ClusterPolicyReport{
				Scope: scope,
			})
		} else {
			report = openreports.NewWGPolAdapter(&v1alpha2.PolicyReport{
				Scope: scope,
			})
		}
	}

	report.SetName(name)
	report.SetNamespace(namespace)
	SetManagedByKyvernoLabel(report)
	SetResults(report, results...)
	return report
}
