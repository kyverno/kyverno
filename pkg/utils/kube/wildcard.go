package kube

import (
	stringutils "github.com/kyverno/kyverno/pkg/utils/string"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func LabelSelectorContainsWildcard(v *metav1.LabelSelector) bool {
	for k, v := range v.MatchLabels {
		if stringutils.ContainsWildcard(k) || stringutils.ContainsWildcard(v) {
			return true
		}
	}
	return false
}
