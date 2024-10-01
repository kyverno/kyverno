package ttl

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mock metav1.Object for testing
type mockMetaObject struct {
	metav1.ObjectMeta
}

func (m *mockMetaObject) GetAnnotations() map[string]string {
	return m.Annotations
}

// Function to test the determinePropagationPolicy function
func TestDeterminePropagationPolicy(t *testing.T) {
	// Set up a mock logger (you can use logr.Discard() for no-op logging)
	logger := logr.Discard()

	// Test cases
	tests := []struct {
		name           string
		annotations    map[string]string
		expectedPolicy *metav1.DeletionPropagation
	}{
		{
			name: "Foreground policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Foreground",
			},
			expectedPolicy: func() *metav1.DeletionPropagation {
				fg := metav1.DeletePropagationForeground
				return &fg
			}(),
		},
		{
			name: "Background policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Background",
			},
			expectedPolicy: func() *metav1.DeletionPropagation {
				bg := metav1.DeletePropagationBackground
				return &bg
			}(),
		},
		{
			name: "Orphan policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Orphan",
			},
			expectedPolicy: func() *metav1.DeletionPropagation {
				orphan := metav1.DeletePropagationOrphan
				return &orphan
			}(),
		},
		{
			name:           "No annotation set",
			annotations:    map[string]string{},
			expectedPolicy: nil,
		},
		{
			name: "Unknown annotation",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "UnknownPolicy",
			},
			expectedPolicy: nil, // Expect nil for unknown policies
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock meta object with the annotations
			metaObj := &mockMetaObject{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tc.annotations,
				},
			}

			// Create the controller instance
			c := &controller{}

			// Call the function to test
			policy := c.determinePropagationPolicy(metaObj, logger)

			// Assert the results
			assert.Equal(t, tc.expectedPolicy, policy)
		})
	}
}
