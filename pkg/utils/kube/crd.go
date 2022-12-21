package kube

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type disco interface {
	GetGVRFromKind(string) (schema.GroupVersionResource, error)
}

// CRDsInstalled checks if the Kyverno CRDs are installed or not
func CRDsInstalled(discovery disco) bool {
	kyvernoCRDs := []string{"ClusterPolicy", "ClusterPolicyReport", "PolicyReport", "AdmissionReport", "BackgroundScanReport", "ClusterAdmissionReport", "ClusterBackgroundScanReport"}
	for _, crd := range kyvernoCRDs {
		if !isCRDInstalled(discovery, crd) {
			return false
		}
	}
	return true
}

func isCRDInstalled(discovery disco, kind string) bool {
	gvr, err := discovery.GetGVRFromKind(kind)
	if gvr.Empty() {
		if err == nil {
			err = fmt.Errorf("not found")
		}
		logging.Error(err, "failed to retrieve CRD", "kind", kind)
		return false
	}
	return true
}
