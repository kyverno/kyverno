package common

import (
	"encoding/json"
	"fmt"

	enginutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func GetNamespaceSelectors(request *v1beta1.AdmissionRequest, resource *unstructured.Unstructured, nsLister listerv1.NamespaceLister) map[string]string {
	var namespaceLabels map[string]string
	var kind, namespaceOfResource string

	if request != nil {
		kind = request.Kind.Kind
		namespaceOfResource = request.Namespace
	} else if resource != nil {
		kind = resource.GetKind()
		namespaceOfResource = resource.GetNamespace()
	}

	if kind != "Namespace" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			log.Log.Error(err, "failed to get the namespace", "name", namespaceOfResource)
		}

		namespaceObj.Kind = "Namespace"
		namespaceRaw, err := json.Marshal(namespaceObj)
		namespaceUnstructured, err := enginutils.ConvertToUnstructured(namespaceRaw)
		if err != nil {
			log.Log.Error(err, "failed to convert object resource to unstructured format")
		}
		fmt.Println("namespaceUnstructured  ", namespaceUnstructured)
		namespaceLabels = namespaceUnstructured.GetLabels()
	}

	return namespaceLabels
}
