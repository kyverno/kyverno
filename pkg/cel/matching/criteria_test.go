package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestMatchCriteria_GetParsedNamespaceSelector(t *testing.T) {
	tests := []struct {
		name        string
		constraints *admissionregistrationv1.MatchResources
		want        labels.Selector
		wantErr     bool
	}{{
		name: "nil",
		constraints: &admissionregistrationv1.MatchResources{
			NamespaceSelector: nil,
		},
		want:    labels.Everything(),
		wantErr: false,
	}, {
		name: "valid",
		constraints: &admissionregistrationv1.MatchResources{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
		want:    labels.SelectorFromSet(map[string]string{"foo": "bar"}),
		wantErr: false,
	}, {
		name: "invalid",
		constraints: &admissionregistrationv1.MatchResources{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo+": "bar",
				},
			},
		},
		want:    nil,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MatchCriteria{
				Constraints: tt.constraints,
			}
			got, err := m.GetParsedNamespaceSelector()
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMatchCriteria_GetParsedObjectSelector(t *testing.T) {
	tests := []struct {
		name        string
		constraints *admissionregistrationv1.MatchResources
		want        labels.Selector
		wantErr     bool
	}{{
		name: "nil",
		constraints: &admissionregistrationv1.MatchResources{
			ObjectSelector: nil,
		},
		want:    labels.Everything(),
		wantErr: false,
	}, {
		name: "valid",
		constraints: &admissionregistrationv1.MatchResources{
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
		want:    labels.SelectorFromSet(map[string]string{"foo": "bar"}),
		wantErr: false,
	}, {
		name: "invalid",
		constraints: &admissionregistrationv1.MatchResources{
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo+": "bar",
				},
			},
		},
		want:    nil,
		wantErr: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MatchCriteria{
				Constraints: tt.constraints,
			}
			got, err := m.GetParsedObjectSelector()
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMatchCriteria_GetMatchResources(t *testing.T) {
	tests := []struct {
		name        string
		constraints *admissionregistrationv1.MatchResources
		want        admissionregistrationv1.MatchResources
	}{{
		name: "test",
		constraints: &admissionregistrationv1.MatchResources{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v1"},
						Resources:   []string{"foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v2"},
						Resources:   []string{"bars", "foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
		want: admissionregistrationv1.MatchResources{
			NamespaceSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			ObjectSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v1"},
						Resources:   []string{"foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"flop"},
						APIVersions: []string{"v2"},
						Resources:   []string{"bars", "foos"},
					},
				},
			}, {
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{"foo"},
						APIVersions: []string{"v1"},
						Resources:   []string{"bars"},
					},
				},
			}},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MatchCriteria{
				Constraints: tt.constraints,
			}
			got := m.GetMatchResources()
			assert.Equal(t, tt.want, got)
		})
	}
}
func TestMatchCriteria_GetParsedSelectorWithNilMatchResources(t *testing.T) {
	tests := []struct {
		name           string
		constraints    *admissionregistrationv1.MatchResources
		selectorGetter func(m *MatchCriteria) (labels.Selector, error)
		selectorField  string // To help identify the field (NamespaceSelector vs ObjectSelector)
		want           labels.Selector
		wantErr        bool
	}{
		{
			name:        "nil MatchResources",
			constraints: nil, // No MatchResources
			selectorGetter: func(m *MatchCriteria) (labels.Selector, error) {
				return m.GetParsedNamespaceSelector() // Can be swapped with ObjectSelector
			},
			selectorField: "NamespaceSelector",
			want:          labels.Everything(), // Expecting Everything() for nil selector
			wantErr:       false,
		},
		{
			name: "nil NamespaceSelector in MatchResources",
			constraints: &admissionregistrationv1.MatchResources{
				NamespaceSelector: nil, // Only this is nil, rest exist
			},
			selectorGetter: func(m *MatchCriteria) (labels.Selector, error) {
				return m.GetParsedNamespaceSelector()
			},
			selectorField: "NamespaceSelector",
			want:          labels.Everything(), // Expecting Everything() for nil NamespaceSelector
			wantErr:       false,
		},
		{
			name: "nil ObjectSelector in MatchResources",
			constraints: &admissionregistrationv1.MatchResources{
				ObjectSelector: nil, // Only ObjectSelector is nil
			},
			selectorGetter: func(m *MatchCriteria) (labels.Selector, error) {
				return m.GetParsedObjectSelector()
			},
			selectorField: "ObjectSelector",
			want:          labels.Everything(), // Expecting Everything() for nil ObjectSelector
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MatchCriteria{
				Constraints: tt.constraints,
			}
			got, err := tt.selectorGetter(m)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
