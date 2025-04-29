package matching

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MatchCriteria struct {
	Constraints *admissionregistrationv1.MatchResources
}

// GetParsedNamespaceSelector returns the converted LabelSelector which implements labels.Selector
func (m *MatchCriteria) GetParsedNamespaceSelector() (labels.Selector, error) {
	if m.Constraints.NamespaceSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(m.Constraints.NamespaceSelector)
}

// GetParsedObjectSelector returns the converted LabelSelector which implements labels.Selector
func (m *MatchCriteria) GetParsedObjectSelector() (labels.Selector, error) {
	if m.Constraints.ObjectSelector == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(m.Constraints.ObjectSelector)
}

// GetMatchResources returns the matchConstraints
func (m *MatchCriteria) GetMatchResources() admissionregistrationv1.MatchResources {
	return *m.Constraints
}
