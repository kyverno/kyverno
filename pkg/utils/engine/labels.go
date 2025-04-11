package engine

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// GetNamespaceSelectorsFromNamespaceLister - extract the namespacelabels when namespace lister is passed
func GetNamespaceSelectorsFromNamespaceLister(kind, namespaceOfResource string, nsLister corev1listers.NamespaceLister, policies []kyvernov1.PolicyInterface, logger logr.Logger) map[string]string {
	namespaceLabels := make(map[string]string)
	if kind != "Namespace" && namespaceOfResource != "" {
		namespaceObj, err := nsLister.Get(namespaceOfResource)
		if err != nil {
			logging.Error(err, "failed to get the namespace", "name", namespaceOfResource)
			return namespaceLabels
		}
		return namespaceObj.DeepCopy().GetLabels()
	}
	return namespaceLabels
}

func HasNamespaceSelector(policies []kyvernov1.PolicyInterface) bool {
	for _, policy := range policies {
		spec := policy.GetSpec()
		if spec == nil {
			continue
		}

		rules := spec.GetRules()
		for _, rule := range rules {
			if rule.MatchResources.ResourceDescription.NamespaceSelector != nil {
				return true
			}

			if rule.ExcludeResources != nil && rule.ExcludeResources.ResourceDescription.NamespaceSelector != nil {
				return true
			}
		}
	}

	return false
}
