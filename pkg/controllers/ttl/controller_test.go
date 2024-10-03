package ttl

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
)

// TestDeterminePropagationPolicy tests the determinePropagationPolicy function
func TestDeterminePropagationPolicy(t *testing.T) {
	logger := logr.Discard() // Use a no-op logger

	testCases := []struct {
		name           string
		annotations    map[string]string
		expectedPolicy *metav1.DeletionPropagation
	}{
		{
			name:        "No annotations",
			annotations: nil,
			expectedPolicy: nil,
		},
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
			name: "Empty annotation",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "",
			},
			expectedPolicy: nil,
		},
		{
			name: "Unknown policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Unknown",
			},
			expectedPolicy: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock metadata object with annotations
			metaObj := &metav1.ObjectMeta{
				Annotations: tc.annotations,
			}

			// Call the function
			policy := determinePropagationPolicy(metaObj, logger)

			// Assert the result
			assert.Equal(t, tc.expectedPolicy, policy)
		})
	}
}
