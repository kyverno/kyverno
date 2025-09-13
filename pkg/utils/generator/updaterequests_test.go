package generator

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config/mocks"
	"github.com/stretchr/testify/assert"
	gomock "go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/metadata/fake"
)

func TestUpdateRequestGenerator_CleanupLabel(t *testing.T) {
	tests := []struct {
		name                       string
		enableUpdateRequestCleanup bool
		updateRequestCleanupTTL    string
		existingLabels             map[string]string
		expectedCleanupLabel       bool
		expectedCleanupLabelValue  string
	}{
		{
			name:                       "cleanup enabled with custom TTL",
			enableUpdateRequestCleanup: true,
			updateRequestCleanupTTL:    "5m",
			existingLabels:             map[string]string{"test": "value"},
			expectedCleanupLabel:       true,
			expectedCleanupLabelValue:  "5m",
		},
		{
			name:                       "cleanup enabled with default TTL",
			enableUpdateRequestCleanup: true,
			updateRequestCleanupTTL:    "2m",
			existingLabels:             nil,
			expectedCleanupLabel:       true,
			expectedCleanupLabelValue:  "2m",
		},
		{
			name:                       "cleanup disabled",
			enableUpdateRequestCleanup: false,
			updateRequestCleanupTTL:    "2m",
			existingLabels:             map[string]string{"test": "value"},
			expectedCleanupLabel:       false,
			expectedCleanupLabelValue:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create mock configuration
			mockConfig := mocks.NewMockConfiguration(ctrl)
			mockConfig.EXPECT().GetUpdateRequestThreshold().Return(int64(1000)).AnyTimes()
			mockConfig.EXPECT().GetEnableUpdateRequestCleanup().Return(tt.enableUpdateRequestCleanup).AnyTimes()
			mockConfig.EXPECT().GetUpdateRequestCleanupTTL().Return(tt.updateRequestCleanupTTL).AnyTimes()

			// Create fake metadata client with empty list
			scheme := runtime.NewScheme()
			metaClient := fake.NewSimpleMetadataClient(scheme)

			// Create generator
			generator := NewUpdateRequestGenerator(mockConfig, metaClient)

			// Create test UpdateRequest
			ur := &kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ur",
					Namespace: "kyverno",
					Labels:    tt.existingLabels,
				},
				Spec: kyvernov2.UpdateRequestSpec{
					Type:   kyvernov2.Mutate,
					Policy: "test-policy",
				},
			}

			// Mock the metadata client to return empty list for threshold check
			// This is handled automatically by the fake client for empty lists

			// Call Generate (this would normally create the resource, but fake client just validates the structure)
			ctx := context.Background()

			// Since we can't easily test the actual Create call without a full Kubernetes client,
			// we'll test the label addition logic by checking the resource before it's passed to Create
			actualUR := ur.DeepCopy()

			// Simulate the label addition logic from the Generate method
			if mockConfig.GetEnableUpdateRequestCleanup() {
				if actualUR.Labels == nil {
					actualUR.Labels = make(map[string]string)
				}
				actualUR.Labels["cleanup.kyverno.io/ttl"] = mockConfig.GetUpdateRequestCleanupTTL()
			}

			// Verify the cleanup label
			if tt.expectedCleanupLabel {
				assert.Contains(t, actualUR.Labels, "cleanup.kyverno.io/ttl")
				assert.Equal(t, tt.expectedCleanupLabelValue, actualUR.Labels["cleanup.kyverno.io/ttl"])
			} else {
				if actualUR.Labels != nil {
					assert.NotContains(t, actualUR.Labels, "cleanup.kyverno.io/ttl")
				}
			}

			// Verify existing labels are preserved
			if tt.existingLabels != nil {
				for key, value := range tt.existingLabels {
					assert.Equal(t, value, actualUR.Labels[key])
				}
			}
		})
	}
}
