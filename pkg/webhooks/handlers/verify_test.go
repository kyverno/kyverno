package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestVerify(t *testing.T) {
	kyvernoNamespace := config.KyvernoNamespace()
	ctx := context.Background()
	logger := logr.Discard()
	startTime := time.Now()

	tests := []struct {
		name           string
		requestName    string
		requestNS      string
		expectMutation bool
		expectAllowed  bool
		expectPatchOp  string
	}{
		{
			name:           "kyverno-health in kyverno namespace should mutate",
			requestName:    "kyverno-health",
			requestNS:      kyvernoNamespace,
			expectMutation: true,
			expectAllowed:  true,
			expectPatchOp:  "replace",
		},
		{
			name:           "different resource name should not mutate",
			requestName:    "some-other-resource",
			requestNS:      kyvernoNamespace,
			expectMutation: false,
			expectAllowed:  true,
		},
		{
			name:           "kyverno-health in different namespace should not mutate",
			requestName:    "kyverno-health",
			requestNS:      "default",
			expectMutation: false,
			expectAllowed:  true,
		},
		{
			name:           "different resource in different namespace should not mutate",
			requestName:    "some-resource",
			requestNS:      "default",
			expectMutation: false,
			expectAllowed:  true,
		},
		{
			name:           "kyverno-health with empty namespace should not mutate",
			requestName:    "kyverno-health",
			requestNS:      "",
			expectMutation: false,
			expectAllowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid-" + tt.name),
					Name:      tt.requestName,
					Namespace: tt.requestNS,
					Kind: metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "ConfigMap",
					},
					Resource: metav1.GroupVersionResource{
						Group:    "",
						Version:  "v1",
						Resource: "configmaps",
					},
					Operation: admissionv1.Update,
				},
			}

			response := Verify(ctx, logger, request, startTime)

			assert.Equal(t, request.UID, response.UID, "response UID should match request UID")
			assert.Equal(t, tt.expectAllowed, response.Allowed, "response allowed status should match expected")

			if tt.expectMutation {
				assert.NotNil(t, response.Patch, "patch should not be nil for mutation")
				assert.NotEmpty(t, response.Patch, "patch should not be empty for mutation")

				var patches []map[string]interface{}
				err := json.Unmarshal(response.Patch, &patches)
				assert.NoError(t, err, "patch should be valid JSON")
				assert.Len(t, patches, 1, "should have exactly one patch operation")

				if len(patches) > 0 {
					patch := patches[0]
					assert.Equal(t, tt.expectPatchOp, patch["op"], "patch operation should be replace")
					assert.Equal(t, "/metadata/annotations/kyverno.io~1last-request-time", patch["path"], "patch path should target last-request-time annotation")

					value, ok := patch["value"].(string)
					assert.True(t, ok, "patch value should be a string")
					_, err := time.Parse(time.RFC3339, value)
					assert.NoError(t, err, "patch value should be a valid RFC3339 timestamp")
				}

				assert.Equal(t, admissionv1.PatchTypeJSONPatch, *response.PatchType, "patch type should be JSONPatch")
			} else {
				assert.Nil(t, response.Patch, "patch should be nil when no mutation expected")
			}
		})
	}
}

func TestVerify_PatchTimestamp(t *testing.T) {
	kyvernoNamespace := config.KyvernoNamespace()
	ctx := context.Background()
	logger := logr.Discard()

	request := AdmissionRequest{
		AdmissionRequest: admissionv1.AdmissionRequest{
			UID:       types.UID("test-uid"),
			Name:      "kyverno-health",
			Namespace: kyvernoNamespace,
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "ConfigMap",
			},
		},
	}

	beforeTime := time.Now()
	response := Verify(ctx, logger, request, beforeTime)
	afterTime := time.Now()

	assert.True(t, response.Allowed, "response should be allowed")
	assert.NotNil(t, response.Patch, "patch should not be nil")

	var patches []map[string]interface{}
	err := json.Unmarshal(response.Patch, &patches)
	assert.NoError(t, err)
	assert.Len(t, patches, 1)

	if len(patches) > 0 {
		value, ok := patches[0]["value"].(string)
		assert.True(t, ok)

		patchTime, err := time.Parse(time.RFC3339, value)
		assert.NoError(t, err)

		assert.True(t, patchTime.After(beforeTime.Add(-time.Second)), "patch time should be after before time")
		assert.True(t, patchTime.Before(afterTime.Add(time.Second)), "patch time should be before after time")
	}
}

func TestVerify_CaseSensitivity(t *testing.T) {
	kyvernoNamespace := config.KyvernoNamespace()
	ctx := context.Background()
	logger := logr.Discard()
	startTime := time.Now()

	tests := []struct {
		name         string
		resourceName string
		expectMutate bool
	}{
		{
			name:         "exact match should mutate",
			resourceName: "kyverno-health",
			expectMutate: true,
		},
		{
			name:         "uppercase name should not mutate",
			resourceName: "KYVERNO-HEALTH",
			expectMutate: false,
		},
		{
			name:         "mixed case should not mutate",
			resourceName: "Kyverno-Health",
			expectMutate: false,
		},
		{
			name:         "with trailing space should not mutate",
			resourceName: "kyverno-health ",
			expectMutate: false,
		},
		{
			name:         "with leading space should not mutate",
			resourceName: " kyverno-health",
			expectMutate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := AdmissionRequest{
				AdmissionRequest: admissionv1.AdmissionRequest{
					UID:       types.UID("test-uid"),
					Name:      tt.resourceName,
					Namespace: kyvernoNamespace,
					Kind: metav1.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "ConfigMap",
					},
				},
			}

			response := Verify(ctx, logger, request, startTime)

			if tt.expectMutate {
				assert.NotNil(t, response.Patch, "should have patch for exact match")
			} else {
				assert.Nil(t, response.Patch, "should not have patch for non-exact match")
			}
		})
	}
}
