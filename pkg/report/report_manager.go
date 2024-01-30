package report

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
)

type reportManager struct {
	storeInDB bool
	client    versioned.Interface
}

type Interface interface {
	CreateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error)
	UpdateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error)
	DeleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) error

	NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface
	BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface
	NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface

	GetAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (runtime.Object, error)
	DeleteAdmissionReports(ctx context.Context, name, namespace string, opts metav1.DeleteOptions) error

	GetBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (runtime.Object, error)
	DeleteBackgroundScanReports(ctx context.Context, name, namespace string, opts metav1.DeleteOptions) error

	GetClusterAdmissionReports(ctx context.Context, name string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterAdmissionReports(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error)
	DeleteClusterAdmissionReports(ctx context.Context, namespace string, opts metav1.DeleteOptions) error

	GetClusterBackgroundScanReports(ctx context.Context, name string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterBackgroundScanReports(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error)
	DeleteClusterBackgroundScanReports(ctx context.Context, namespace string, opts metav1.DeleteOptions) error

	AdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	ClusterAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	BackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	ClusterBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
}

func NewReportManager(storeInDB bool, client versioned.Interface) Interface {
	return &reportManager{
		storeInDB: storeInDB,
		client:    client,
	}
}

func (r *reportManager) CreateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error) {
	return reportutils.CreateReport(ctx, report, r.client)
}

func (r *reportManager) UpdateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error) {
	return reportutils.UpdateReport(ctx, report, r.client)
}

func (r *reportManager) DeleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) error {
	return reportutils.DeleteReport(ctx, report, r.client)
}

func (r *reportManager) GetAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(ctx, name, opts)
	}
}

func (r *reportManager) ListAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().AdmissionReports(namespace).List(ctx, opts)
	}
}

func (r *reportManager) DeleteAdmissionReports(ctx context.Context, name, namespace string, opts metav1.DeleteOptions) error {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).Delete(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().AdmissionReports(namespace).Delete(ctx, name, opts)
	}
}

func (r *reportManager) GetBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().BackgroundScanReports(namespace).Get(ctx, name, opts)
	}
}

func (r *reportManager) ListBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (runtime.Object, error) {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().BackgroundScanReports(namespace).List(ctx, opts)
	}
}

func (r *reportManager) DeleteBackgroundScanReports(ctx context.Context, name, namespace string, opts metav1.DeleteOptions) error {
	if r.storeInDB {
		return r.client.ReportsV1().EphemeralReports(namespace).Delete(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().BackgroundScanReports(namespace).Delete(ctx, name, opts)
	}
}

func (r *reportManager) GetClusterAdmissionReports(ctx context.Context, name string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(ctx, name, opts)
	}
}

func (r *reportManager) ListClusterAdmissionReports(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterAdmissionReports().List(ctx, opts)
	}
}

func (r *reportManager) DeleteClusterAdmissionReports(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().Delete(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterAdmissionReports().Delete(ctx, name, opts)
	}
}

func (r *reportManager) GetClusterBackgroundScanReports(ctx context.Context, name string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterBackgroundScanReports().Get(ctx, name, opts)
	}
}

func (r *reportManager) ListClusterBackgroundScanReports(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterBackgroundScanReports().List(ctx, opts)
	}
}

func (r *reportManager) DeleteClusterBackgroundScanReports(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterEphemeralReports().Delete(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterBackgroundScanReports().Delete(ctx, name, opts)
	}
}

func (r *reportManager) NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return NewAdmissionReport(namespace, name, gvr, resource)
	} else {
		return newAdmissionReportV1Alpha1(namespace, name, gvr, resource)
	}
}

func (r *reportManager) BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return BuildAdmissionReport(resource, request, responses...)
	} else {
		return buildAdmissionReportV1Alpha1(resource, request, responses...)
	}
}

func (r *reportManager) NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return NewBackgroundScanReport(namespace, name, gvk, owner, uid)
	} else {
		return newBackgroundScanReportV1Alpha1(namespace, name, gvk, owner, uid)
	}
}

func (r *reportManager) AdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("admissionreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	}
}

func (r *reportManager) ClusterAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	}
}

func (r *reportManager) BackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("backgroundscanreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"))
	}
}

func (r *reportManager) ClusterBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	}
}
