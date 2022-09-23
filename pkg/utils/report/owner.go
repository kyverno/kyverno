package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func SetOwner(report kyvernov1alpha2.ReportChangeRequestInterface, group, version, kind string, name string, uid types.UID) {
	gv := metav1.GroupVersion{Group: group, Version: version}
	controllerutils.SetOwner(report, gv.String(), kind, name, uid)
}
