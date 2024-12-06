package ttl

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
			name:           "No annotations",
			annotations:    nil,
			expectedPolicy: nil,
		},
		{
			name: "Foreground policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Foreground",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationForeground),
		},
		{
			name: "Background policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Background",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationBackground),
		},
		{
			name: "Orphan policy",
			annotations: map[string]string{
				kyverno.AnnotationCleanupPropagationPolicy: "Orphan",
			},
			expectedPolicy: ptr.To(metav1.DeletePropagationOrphan),
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
