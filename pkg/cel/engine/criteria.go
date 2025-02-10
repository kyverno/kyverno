package engine

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type matchCriteria struct {
	constraints *admissionregistrationv1.MatchResources
}

// GetParsedNamespaceSelector returns the converted LabelSelector which implements labels.Selector
func (m *matchCriteria) GetParsedNamespaceSelector() (labels.Selector, error) {
	if m.constraints.NamespaceSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(m.constraints.NamespaceSelector)
}

// GetParsedObjectSelector returns the converted LabelSelector which implements labels.Selector
func (m *matchCriteria) GetParsedObjectSelector() (labels.Selector, error) {
	if m.constraints.ObjectSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(m.constraints.ObjectSelector)
}

// GetMatchResources returns the matchConstraints
func (m *matchCriteria) GetMatchResources() admissionregistrationv1.MatchResources {
	return *m.constraints
}
