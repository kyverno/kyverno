package validation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func matchResource(resource unstructured.Unstructured, rule kyvernov1.Rule, admissionInfo kyvernov2.RequestInfo, namespaceLabels map[string]string, policyNamespace string, operation kyvernov1.AdmissionOperation) bool {
	err := engineutils.MatchesResourceDescription(
		resource,
		rule,
		admissionInfo,
		namespaceLabels,
		policyNamespace,
		resource.GroupVersionKind(),
		"",
		operation,
	)
	return err == nil
}
