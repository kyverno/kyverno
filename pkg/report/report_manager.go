package report

import (
	"context"

	reportv1 "github.com/kyverno/kyverno/api/kyverno/reports/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	ListAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error)

	GetBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error)

	GetClusterAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error)

	GetClusterBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error)

	GetAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	GetClusterAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	GetBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer
	GetClusterBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer

	DeepCopy(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface
}

func NewReportManager(storeInDB bool, client versioned.Interface) Interface {
	return &reportManager{
		storeInDB: storeInDB,
		client:    client,
	}
}

func (r *reportManager) CreateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return createReportV1Report(ctx, report, r.client)
	} else {
		return createV1Alpha1Report(ctx, report, r.client)
	}
}

func (r *reportManager) UpdateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return updateReportsV1Report(ctx, report, r.client)
	} else {
		return updateV1Alpha1Report(ctx, report, r.client)
	}
}

func (r *reportManager) DeleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface) error {
	if r.storeInDB {
		return deleteReportV1Reports(ctx, report, r.client)
	} else {
		return deleteV1Alpha1Reports(ctx, report, r.client)
	}
}

func (r *reportManager) GetAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().AdmissionReports(namespace).Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().AdmissionReports(namespace).Get(ctx, name, opts)
	}
}

func (r *reportManager) ListAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().AdmissionReports(namespace).List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().AdmissionReports(namespace).List(ctx, opts)
	}
}

func (r *reportManager) GetBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().BackgroundScanReports(namespace).Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().BackgroundScanReports(namespace).Get(ctx, name, opts)
	}
}

func (r *reportManager) ListBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().BackgroundScanReports(namespace).List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().BackgroundScanReports(namespace).List(ctx, opts)
	}
}

func (r *reportManager) GetClusterAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterAdmissionReports().Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterAdmissionReports().Get(ctx, name, opts)
	}
}

func (r *reportManager) ListClusterAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterAdmissionReports().List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterAdmissionReports().List(ctx, opts)
	}
}

func (r *reportManager) GetClusterBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterBackgroundScanReports().Get(ctx, name, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterBackgroundScanReports().Get(ctx, name, opts)
	}
}

func (r *reportManager) ListClusterBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportListInterface, error) {
	if r.storeInDB {
		return r.client.ReportsV1().ClusterBackgroundScanReports().List(ctx, opts)
	} else {
		return r.client.KyvernoV1alpha2().ClusterBackgroundScanReports().List(ctx, opts)
	}
}

func (r *reportManager) NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return newAdmissionReportReportV1(namespace, name, gvr, resource)
	} else {
		return newAdmissionReportV1Alpha1(namespace, name, gvr, resource)
	}
}

func (r *reportManager) BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return buildAdmissionReportReportV1(resource, request, responses...)
	} else {
		return buildAdmissionReportV1Alpha1(resource, request, responses...)
	}
}

func (r *reportManager) NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return newBackgroundScanReportReportsV1(namespace, name, gvk, owner, uid)
	} else {
		return newBackgroundScanReportReportsV1(namespace, name, gvk, owner, uid)
	}
}

func (r *reportManager) GetAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportv1.SchemeGroupVersion.WithResource("admissionreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("admissionreports"))
	}
}

func (r *reportManager) GetClusterAdmissionReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportv1.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusteradmissionreports"))
	}
}

func (r *reportManager) GetBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportv1.SchemeGroupVersion.WithResource("backgroundscanreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("backgroundscanreports"))
	}
}

func (r *reportManager) GetClusterBackgroundScanReportInformer(metadataFactory metadatainformers.SharedInformerFactory) informers.GenericInformer {
	if r.storeInDB {
		return metadataFactory.ForResource(reportv1.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	} else {
		return metadataFactory.ForResource(kyvernov1alpha2.SchemeGroupVersion.WithResource("clusterbackgroundscanreports"))
	}
}

func (r *reportManager) DeepCopy(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface {
	if r.storeInDB {
		return deepCopyReportV1(report)
	} else {
		return deepCopyV1Alpha1(report)
	}
}
