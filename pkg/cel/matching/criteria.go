package matching

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type MatchCriteria struct {
	Constraints *admissionregistrationv1.MatchResources
}

func (m *MatchCriteria) getSelector(sel *metav1.LabelSelector) (labels.Selector, error) {
	if sel == nil {
		return labels.Everything(), nil
	}
	return metav1.LabelSelectorAsSelector(sel)
}

// GetParsedNamespaceSelector returns the namespace selector or Everything
func (m *MatchCriteria) GetParsedNamespaceSelector() (labels.Selector, error) {
	if m.Constraints == nil {
		return labels.Everything(), nil
	}
	return m.getSelector(m.Constraints.NamespaceSelector)
}

// GetParsedObjectSelector returns the object selector or Everything
func (m *MatchCriteria) GetParsedObjectSelector() (labels.Selector, error) {
	if m.Constraints == nil {
		return labels.Everything(), nil
	}
	return m.getSelector(m.Constraints.ObjectSelector)
}

// GetMatchResources returns a copy of the constraints or an empty struct
func (m *MatchCriteria) GetMatchResources() admissionregistrationv1.MatchResources {
	if m.Constraints == nil {
		return admissionregistrationv1.MatchResources{}
	}
	return *m.Constraints
}
