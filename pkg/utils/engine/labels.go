package engine

import (
	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// GetNamespaceSelectorsFromNamespaceLister - extract the namespacelabels when namespace lister is passed
func GetNamespaceSelectorsFromNamespaceLister(kind, namespaceOfResource string, nsLister corev1listers.NamespaceLister, logger logr.Logger) map[string]string {
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
