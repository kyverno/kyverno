package engine

import (
	"testing"

	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		policies []policiesv1alpha1.MutatingPolicy
		expected []bool
	}{
		{
			name:     "no names provided - always match",
			names:    []string{},
			policies: []policiesv1alpha1.MutatingPolicy{{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "any"}}},
			expected: []bool{true, true},
		},
		{
			name:     "single name match",
			names:    []string{"p1"},
			policies: []policiesv1alpha1.MutatingPolicy{{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "p2"}}},
			expected: []bool{true, false},
		},
		{
			name:     "multiple name match",
			names:    []string{"p1", "p3"},
			policies: []policiesv1alpha1.MutatingPolicy{{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, {ObjectMeta: metav1.ObjectMeta{Name: "p2"}}, {ObjectMeta: metav1.ObjectMeta{Name: "p3"}}},
			expected: []bool{true, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := MatchNames(tt.names...)
			for i, policy := range tt.policies {
				assert.Equal(t, tt.expected[i], predicate(policy), "policy name: %s", policy.Name)
			}
		})
	}
}
