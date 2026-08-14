package test

import (
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

// TestExcludedResourceDisplay verifies that excluded resources display their full path
func TestExcludedResourceDisplay(t *testing.T) {
	color.NoColor = true

	testCases := []struct {
		name            string
		resourceKey     string
		expectedDisplay string
	}{
		{
			name:            "deployment in default namespace",
			resourceKey:     "apps/v1,Deployment,default,skipped-deployment-1",
			expectedDisplay: "apps/v1/Deployment/default/skipped-deployment-1",
		},
		{
			name:            "deployment in staging namespace",
			resourceKey:     "apps/v1,Deployment,staging,skipped-deployment-2",
			expectedDisplay: "apps/v1/Deployment/staging/skipped-deployment-2",
		},
		{
			name:            "pod in kube-system",
			resourceKey:     "v1,Pod,kube-system,test-pod",
			expectedDisplay: "v1/Pod/kube-system/test-pod",
		},
		{
			name:            "service in custom namespace",
			resourceKey:     "v1,Service,my-app,frontend-svc",
			expectedDisplay: "v1/Service/my-app/frontend-svc",
		},
		{
			name:            "clusterrole with empty namespace (cluster-scoped)",
			resourceKey:     "v1,ClusterRole,,admin",
			expectedDisplay: "v1/ClusterRole/admin",
		},
		{
			name:            "simple fallback resource (no commas)",
			resourceKey:     "test-pod",
			expectedDisplay: "/test-pod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatResource(tc.resourceKey)
			assert.Equal(t, tc.expectedDisplay, result)
		})
	}
}

// TestExcludedResourceFormatConsistency verifies the resource format matches the actual output
func TestExcludedResourceFormatConsistency(t *testing.T) {
	color.NoColor = true

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
		{
			name:           "clusterrole cluster-scoped",
			resourceKey:    "v1,ClusterRole,,admin",
			expectedFormat: "v1/ClusterRole/admin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatResource(tc.resourceKey)
			assert.Equal(t, tc.expectedFormat, result)
		})
	}
}
