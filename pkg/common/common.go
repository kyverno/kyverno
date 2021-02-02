package common

import (
	"encoding/json"

	"github.com/go-logr/logr"
	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
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

func GetNamespaceSelectors(kind, namespaceOfResource string, nsLister listerv1.NamespaceLister, logger logr.Logger) map[string]string {
	var namespaceLabels map[string]string
	if kind != "Namespace" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
		}

		namespaceObj.Kind = "Namespace"
		namespaceRaw, err := json.Marshal(namespaceObj)
		namespaceUnstructured, err := enginutils.ConvertToUnstructured(namespaceRaw)
		if err != nil {
			logger.Error(err, "failed to convert object resource to unstructured format")
		}
		namespaceLabels = namespaceUnstructured.GetLabels()
	}

	return namespaceLabels
}
