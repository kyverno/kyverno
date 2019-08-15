package gencontroller

import (
	"github.com/minio/minio/pkg/wildcard"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	wqNamespace  string = "namespace"
	workerCount  int    = 1
	wqRetryLimit int    = 5
	policyKind   string = "Policy"
)

func namespaceMeetsRuleDescription(ns *corev1.Namespace, resourceDescription v1alpha1.ResourceDescription) bool {
	//REWORK Not needed but verify the 'Namespace' is defined in the list of supported kinds
	if !findKind(resourceDescription.Kinds, "Namespace") {
		return false
	}
	if resourceDescription.Name != nil {
		if !wildcard.Match(*resourceDescription.Name, ns.Name) {
			return false
		}
	}

	if resourceDescription.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(resourceDescription.Selector)
		if err != nil {
			return false
		}

		labelSet := convertLabelsToLabelSet(ns.Labels)
		// labels
		if !selector.Matches(labelSet) {
			return false
		}
	}
	return true
}

func convertLabelsToLabelSet(labelMap map[string]string) labels.Set {
	labelSet := make(labels.Set, len(labelMap))
	// REWORK: check if the below works
	// if x, ok := labelMap.(labels.Set); !ok {

	// }
	for k, v := range labelMap {
		labelSet[k] = v
	}
	return labelSet
}

func findKind(kinds []string, kindGVK string) bool {
	for _, kind := range kinds {
		if kind == kindGVK {
			return true
		}
	}
	return false
}
