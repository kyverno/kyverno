package report

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type reportManager struct {
	storeInDB bool
	client    versioned.Interface
}

type Interface interface {
	CreateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error)
	DeleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) error
	UpdateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, client versioned.Interface) (kyvernov1alpha2.ReportInterface, error)

	NewAdmissionReport(namespace, name string, gvr schema.GroupVersionResource, resource unstructured.Unstructured) kyvernov1alpha2.ReportInterface
	BuildAdmissionReport(resource unstructured.Unstructured, request admissionv1.AdmissionRequest, responses ...engineapi.EngineResponse) kyvernov1alpha2.ReportInterface
	NewBackgroundScanReport(namespace, name string, gvk schema.GroupVersionKind, owner string, uid types.UID) kyvernov1alpha2.ReportInterface

	GetAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportInterface, error)

	GetBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportInterface, error)

	GetClusterAdmissionReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterAdmissionReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportInterface, error)

	GetClusterBackgroundScanReports(ctx context.Context, name string, namespace string, opts metav1.GetOptions) (kyvernov1alpha2.ReportInterface, error)
	ListClusterBackgroundScanReports(ctx context.Context, namespace string, opts metav1.ListOptions) (kyvernov1alpha2.ReportInterface, error)

	DeepCopy(report kyvernov1alpha2.ReportInterface) kyvernov1alpha2.ReportInterface
}

func NewReportClient(storeInDB bool, client versioned.Interface) {}
