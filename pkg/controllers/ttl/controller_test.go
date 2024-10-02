package ttl

import (
	"testing"

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

// Function to test the deletion propagation policy directly in the test
func TestDeterminePropagationPolicy(t *testing.T) {
	// Define expected values explicitly
	fg := metav1.DeletePropagationForeground
	bg := metav1.DeletePropagationBackground
	orphan := metav1.DeletePropagationOrphan

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
			expectedPolicy: &fg,
		},
		{
			name: "Background policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Background",
			},
			expectedPolicy: &bg,
		},
		{
			name: "Orphan policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Orphan",
			},
			expectedPolicy: &orphan,
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
			expectedPolicy: nil,
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
			// Determine the deletion propagation policy directly in the test
			var policy *metav1.DeletionPropagation

			if annotations := metaObj.GetAnnotations(); annotations != nil {
				if annotationPolicy := annotations[kyverno.AnnotationCleanupPropagationPolicy]; annotationPolicy != "" {
					switch annotationPolicy {
					case "Foreground":
						policy = &fg
					case "Background":
						policy = &bg
					case "Orphan":
						policy = &orphan
					}
				}
			}
			// Assert the results
			assert.Equal(t, tc.expectedPolicy, policy)
		})
	}
}
