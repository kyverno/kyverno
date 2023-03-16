package kube

import (
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func LabelSelectorContainsWildcard(v *metav1.LabelSelector) bool {
	if v != nil {
		for k, v := range v.MatchLabels {
			if wildcard.ContainsWildcard(k) || wildcard.ContainsWildcard(v) {
				return true
			}
		}
	}
	return false
}
