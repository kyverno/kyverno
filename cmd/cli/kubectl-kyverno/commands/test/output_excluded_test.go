package test

import (
	"testing"

	fatihcolor "github.com/fatih/color"
	outputcolor "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
)

// initNoColor initializes the Kyverno CLI color globals in no-color mode
// and registers a t.Cleanup to restore the previous color state.
func initNoColor(t *testing.T) {
	t.Helper()
	prev := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = prev
		outputcolor.Init(prev)
	})
	fatihcolor.NoColor = true
	outputcolor.Init(true)
}

// TestFormatResource verifies that formatResource correctly converts comma-separated
// resource keys into slash-separated display paths for all key shapes.
func TestFormatResource(t *testing.T) {
	initNoColor(t)

	testCases := []struct {
		name        string
		resourceKey string
		expected    string
	}{
		{
			name:        "namespaced deployment (apps group)",
			resourceKey: "apps/v1,Deployment,default,my-dep",
			expected:    "apps/v1/Deployment/default/my-dep",
		},
		{
			name:        "namespaced pod (core group)",
			resourceKey: "v1,Pod,kube-system,my-pod",
			expected:    "v1/Pod/kube-system/my-pod",
		},
		{
			name:        "cluster-scoped namespace",
			resourceKey: "v1,Namespace,,prod",
			expected:    "v1/Namespace/prod",
		},
		{
			name:        "cluster-scoped clusterrole",
			resourceKey: "rbac.authorization.k8s.io/v1,ClusterRole,,admin",
			expected:    "rbac.authorization.k8s.io/v1/ClusterRole/admin",
		},
		{
			name:        "cluster-scoped CRD",
			resourceKey: "apiextensions.k8s.io/v1,CustomResourceDefinition,,policies.kyverno.io",
			expected:    "apiextensions.k8s.io/v1/CustomResourceDefinition/policies.kyverno.io",
		},
		{
			name:        "fallback: fewer than 4 parts (slash-delimited input)",
			resourceKey: "v1/Pod/default/test-pod",
			expected:    "v1/Pod/default/test-pod",
		},
		{
			name:        "fallback: single component (resource name only)",
			resourceKey: "test-pod",
			expected:    "/test-pod",
		},
		{
			name:        "fallback: surplus components (more than 4 comma parts)",
			resourceKey: "v1,Pod,default,test-pod,extra",
			expected:    "/v1,Pod,default,test-pod,extra",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatResource(tc.resourceKey)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExcludedResourceDisplay verifies that excluded resources display correctly
// for both namespace-scoped and cluster-scoped resources.
func TestExcludedResourceDisplay(t *testing.T) {
	initNoColor(t)

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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatResource(tc.resourceKey)
			assert.Equal(t, tc.expectedDisplay, result)
		})
	}
}

// TestExcludedResourceFormatConsistency verifies the resource format matches
// the actual output for common resource types including cluster-scoped resources.
func TestExcludedResourceFormatConsistency(t *testing.T) {
	initNoColor(t)

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
