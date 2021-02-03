package common

import (
	"encoding/json"

	"github.com/go-logr/logr"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Policy Reporting Modes
const (
	Enforce = "enforce" // blocks the request on failure
	Audit   = "audit"   // dont block the request on failure, but report failiures as policy violations
)

// Policy Reporting Types
const (
	PolicyViolation = "POLICYVIOLATION"
	PolicyReport    = "POLICYREPORT"
)

// GetNamespaceSelectorsFromGenericInformer - extracting the namespacelabels when generic informer is passed
func GetNamespaceSelectorsFromGenericInformer(kind, namespaceOfResource string, nsInformer informers.GenericInformer, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" {
		runtimeNamespaceObj, err := nsInformer.Lister().Get(namespaceOfResource)
		namespaceObj := runtimeNamespaceObj.(*v1.Namespace)

		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
		}
		return GetNamespaceLabels(namespaceObj, logger)
	}

	return namespaceLabels
}

// GetNamespaceSelectorsFromNamespaceLister - extract the namespacelabels when namespace lister is passed
func GetNamespaceSelectorsFromNamespaceLister(kind, namespaceOfResource string, nsLister listerv1.NamespaceLister, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
		}
		return GetNamespaceLabels(namespaceObj, logger)
	}
	return namespaceLabels
}

// GetNamespaceLabels - from namespace obj
func GetNamespaceLabels(namespaceObj *v1.Namespace, logger logr.Logger) map[string]string {
	namespaceObj.Kind = "Namespace"
	namespaceRaw, err := json.Marshal(namespaceObj)
	namespaceUnstructured, err := enginutils.ConvertToUnstructured(namespaceRaw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}
	return namespaceUnstructured.GetLabels()
}
