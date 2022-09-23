package report

import (
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func SetOwner(report metav1.Object, group, version, kind string, name string, uid types.UID) {
	gv := metav1.GroupVersion{Group: group, Version: version}
	controllerutils.SetOwner(report, gv.String(), kind, name, uid)
}
