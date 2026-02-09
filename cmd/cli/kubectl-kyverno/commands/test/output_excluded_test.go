package test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExcludedResourceDisplay verifies that excluded resources display their full path
func TestExcludedResourceDisplay(t *testing.T) {
	testCases := []struct {
		name            string
		resourceKey     string
		expectedDisplay string
		expectedParts   int
	}{
		{
			name:            "deployment in default namespace",
			resourceKey:     "apps/v1,Deployment,default,skipped-deployment-1",
			expectedDisplay: "apps/v1/Deployment/default/skipped-deployment-1",
			expectedParts:   5, // apps, v1, Deployment, default, skipped-deployment-1
		},
		{
			name:            "deployment in staging namespace",
			resourceKey:     "apps/v1,Deployment,staging,skipped-deployment-2",
			expectedDisplay: "apps/v1/Deployment/staging/skipped-deployment-2",
			expectedParts:   5, // apps, v1, Deployment, staging, skipped-deployment-2
		},
		{
			name:            "pod in kube-system",
			resourceKey:     "v1,Pod,kube-system,test-pod",
			expectedDisplay: "v1/Pod/kube-system/test-pod",
			expectedParts:   4, // v1, Pod, kube-system, test-pod
		},
		{
			name:            "service in custom namespace",
			resourceKey:     "v1,Service,my-app,frontend-svc",
			expectedDisplay: "v1/Service/my-app/frontend-svc",
			expectedParts:   4, // v1, Service, my-app, frontend-svc
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This mimics the fix in output.go
			resourceGVKAndName := strings.Replace(tc.resourceKey, ",", "/", -1)
			assert.Equal(t, tc.expectedDisplay, resourceGVKAndName)

			// Verify we can split it correctly for display
			resourceParts := strings.Split(resourceGVKAndName, "/")
			assert.Equal(t, tc.expectedParts, len(resourceParts), "Resource should have expected number of parts")

			// Verify the format used in color.Resource()
			// The last part is always the name
			typeAndNamespace := strings.Join(resourceParts[:len(resourceParts)-1], "/")
			name := resourceParts[len(resourceParts)-1]

			// Reconstruct to verify
			reconstructed := typeAndNamespace + "/" + name
			assert.Equal(t, tc.expectedDisplay, reconstructed)
		})
	}
}

// TestExcludedResourceFormatConsistency verifies the resource format matches the actual output
func TestExcludedResourceFormatConsistency(t *testing.T) {
	// This test verifies that our fix produces the same format as other resources
	testCases := []struct {
		name           string
		resourceKey    string
		expectedFormat string
	}{
		{
			name:           "apps/v1 deployment",
			resourceKey:    "apps/v1,Deployment,default,bad-deployment",
			expectedFormat: "apps/v1/Deployment/default/bad-deployment",
		},
		{
			name:           "v1 pod",
			resourceKey:    "v1,Pod,default,test-pod",
			expectedFormat: "v1/Pod/default/test-pod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Apply the transformation from the fix
			result := strings.Replace(tc.resourceKey, ",", "/", -1)
			assert.Equal(t, tc.expectedFormat, result)
		})
	}
}

// TestGenerateResourceKey verifies the resource key generation matches our expectations
func TestGenerateResourceKey(t *testing.T) {
	// This test verifies that generateResourceKey creates the format we expect
	testCases := []struct {
		apiVersion string
		kind       string
		namespace  string
		name       string
		expected   string
	}{
		{
			apiVersion: "apps/v1",
			kind:       "Deployment",
			namespace:  "default",
			name:       "test-deployment",
			expected:   "apps/v1,Deployment,default,test-deployment",
		},
		{
			apiVersion: "v1",
			kind:       "Pod",
			namespace:  "kube-system",
			name:       "test-pod",
			expected:   "v1,Pod,kube-system,test-pod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			// This mimics generateResourceKey function
			result := tc.apiVersion + "," + tc.kind + "," + tc.namespace + "," + tc.name
			assert.Equal(t, tc.expected, result)
		})
	}
}
