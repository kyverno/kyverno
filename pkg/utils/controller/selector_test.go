package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/labels"
)

func TestSelectorNotManagedByKyverno(t *testing.T) {
	selector, err := SelectorNotManagedByKyverno()
	assert.NoError(t, err)
	assert.Equal(t, "app.kubernetes.io/managed-by!=kyverno", selector.String())

	tests := []struct {
		name     string
		labels   labels.Set
		expected bool
	}{
		{
			name:     "managed by kyverno",
			labels:   labels.Set{"app.kubernetes.io/managed-by": "kyverno"},
			expected: false,
		},
		{
			name:     "not managed by kyverno",
			labels:   labels.Set{"app.kubernetes.io/managed-by": "other"},
			expected: true,
		},
		{
			name:     "no managed-by label",
			labels:   labels.Set{"foo": "bar"},
			expected: true,
		},
		{
			name:     "empty labels",
			labels:   labels.Set{},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, selector.Matches(tt.labels))
		})
	}
}
