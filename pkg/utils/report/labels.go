package report

import (
	"strconv"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	//	admission request labels
	LabelRequestGroup     = "audit.kyverno.io/request.group"
	LabelRequestKind      = "audit.kyverno.io/request.kind"
	LabelRequestName      = "audit.kyverno.io/request.name"
	LabelRequestNamespace = "audit.kyverno.io/request.namespace"
	LabelRequestUid       = "audit.kyverno.io/request.uid"
	LabelRequestVersion   = "audit.kyverno.io/request.version"
	//	resource labels
	LabelResourceGeneration = "audit.kyverno.io/resource.generation"
	LabelResourceName       = "audit.kyverno.io/resource.name"
	LabelResourceNamespace  = "audit.kyverno.io/resource.namespace"
	LabelResourceUid        = "audit.kyverno.io/resource.uid"
	LabelResourceVersion    = "audit.kyverno.io/resource.version"
	//	resource gvk labels
	LabelResourceGvkGroup   = "audit.kyverno.io/resource.gvk.group"
	LabelResourceGvkKind    = "audit.kyverno.io/resource.gvk.kind"
	LabelResourceGvkVersion = "audit.kyverno.io/resource.gvk.version"
)

func SetManagedByKyvernoLabel(obj metav1.Object) {
	controllerutils.SetLabel(obj, kyvernov1.LabelAppManagedBy, kyvernov1.ValueKyvernoApp)
}

func SetAdmissionLabels(report kyvernov1alpha2.ReportChangeRequestInterface, request *admissionv1.AdmissionRequest) {
	controllerutils.SetLabel(report, LabelRequestGroup, request.Kind.Group)
	controllerutils.SetLabel(report, LabelRequestKind, request.Kind.Kind)
	controllerutils.SetLabel(report, LabelRequestName, request.Name)
	controllerutils.SetLabel(report, LabelRequestNamespace, request.Namespace)
	controllerutils.SetLabel(report, LabelRequestUid, string(request.UID))
	controllerutils.SetLabel(report, LabelRequestVersion, request.Kind.Version)
}

func SetResourceLabels(report kyvernov1alpha2.ReportChangeRequestInterface, resource metav1.Object) {
	controllerutils.SetLabel(report, LabelResourceGeneration, strconv.FormatInt(resource.GetGeneration(), 10))
	controllerutils.SetLabel(report, LabelResourceName, resource.GetName())
	controllerutils.SetLabel(report, LabelResourceNamespace, resource.GetNamespace())
	controllerutils.SetLabel(report, LabelResourceUid, string(resource.GetUID()))
	controllerutils.SetLabel(report, LabelResourceVersion, resource.GetResourceVersion())
}

func SetResourceGvkLabels(report kyvernov1alpha2.ReportChangeRequestInterface, group, version, kind string) {
	controllerutils.SetLabel(report, LabelResourceGvkGroup, group)
	controllerutils.SetLabel(report, LabelResourceGvkKind, kind)
	controllerutils.SetLabel(report, LabelResourceGvkVersion, version)
}
