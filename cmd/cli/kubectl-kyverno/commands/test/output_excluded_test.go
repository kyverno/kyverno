package test

import (
	"testing"

	fatihcolor "github.com/fatih/color"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/output/color"
	"github.com/stretchr/testify/assert"
)

// TestFormatResourceDisplay verifies that formatResourceDisplay correctly parses
// kind, namespace, and name from comma-separated and slash-separated resource keys.
func TestFormatResourceDisplay(t *testing.T) {
	testCases := []struct {
		name              string
		resourceKey       string
		expectedKind      string
		expectedNamespace string
		expectedName      string
	}{
		{
			name:              "deployment in default namespace (comma-separated)",
			resourceKey:       "apps/v1,Deployment,default,skipped-deployment-1",
			expectedKind:      "apps/v1/Deployment",
			expectedNamespace: "default",
			expectedName:      "skipped-deployment-1",
		},
		{
			name:              "deployment in staging namespace (comma-separated)",
			resourceKey:       "apps/v1,Deployment,staging,skipped-deployment-2",
			expectedKind:      "apps/v1/Deployment",
			expectedNamespace: "staging",
			expectedName:      "skipped-deployment-2",
		},
		{
			name:              "pod in kube-system (comma-separated)",
			resourceKey:       "v1,Pod,kube-system,test-pod",
			expectedKind:      "v1/Pod",
			expectedNamespace: "kube-system",
			expectedName:      "test-pod",
		},
		{
			name:              "service in custom namespace (comma-separated)",
			resourceKey:       "v1,Service,my-app,frontend-svc",
			expectedKind:      "v1/Service",
			expectedNamespace: "my-app",
			expectedName:      "frontend-svc",
		},
		{
			name:              "cluster-scoped cluster role (comma-separated)",
			resourceKey:       "rbac.authorization.k8s.io/v1,ClusterRole,,admin",
			expectedKind:      "rbac.authorization.k8s.io/v1/ClusterRole",
			expectedNamespace: "",
			expectedName:      "admin",
		},
		{
			name:              "cluster-scoped resource with group/version slash form",
			resourceKey:       "rbac.authorization.k8s.io/v1/ClusterRole/admin",
			expectedKind:      "rbac.authorization.k8s.io/v1/ClusterRole",
			expectedNamespace: "",
			expectedName:      "admin",
		},
		{
			name:              "namespaced resource starting with v (vault/Pod/my-pod)",
			resourceKey:       "vault/Pod/my-pod",
			expectedKind:      "Pod",
			expectedNamespace: "vault",
			expectedName:      "my-pod",
		},
		{
			name:              "slash-separated 4 parts (v1/Pod/default/test-pod)",
			resourceKey:       "v1/Pod/default/test-pod",
			expectedKind:      "v1/Pod",
			expectedNamespace: "default",
			expectedName:      "test-pod",
		},
		{
			name:              "namespace-first slash 4 parts (default/v1/Pod/test-pod)",
			resourceKey:       "default/v1/Pod/test-pod",
			expectedKind:      "v1/Pod",
			expectedNamespace: "default",
			expectedName:      "test-pod",
		},
		{
			name:              "slash-separated 5 parts (apps/v1/Deployment/default/test-dep)",
			resourceKey:       "apps/v1/Deployment/default/test-dep",
			expectedKind:      "apps/v1/Deployment",
			expectedNamespace: "default",
			expectedName:      "test-dep",
		},
		{
			name:              "namespace-first slash 5 parts (default/apps/v1/Deployment/test-dep)",
			resourceKey:       "default/apps/v1/Deployment/test-dep",
			expectedKind:      "apps/v1/Deployment",
			expectedNamespace: "default",
			expectedName:      "test-dep",
		},
		{
			name:              "slash-separated 3 parts (default/Pod/test-pod)",
			resourceKey:       "default/Pod/test-pod",
			expectedKind:      "Pod",
			expectedNamespace: "default",
			expectedName:      "test-pod",
		},
		{
			name:              "slash-separated 3 parts with apiVersion (v1/ClusterRole/admin)",
			resourceKey:       "v1/ClusterRole/admin",
			expectedKind:      "v1/ClusterRole",
			expectedNamespace: "",
			expectedName:      "admin",
		},
		{
			name:              "slash-separated 2 parts (ClusterRole/admin)",
			resourceKey:       "ClusterRole/admin",
			expectedKind:      "ClusterRole",
			expectedNamespace: "",
			expectedName:      "admin",
		},
		{
			name:              "single part (payload.json)",
			resourceKey:       "payload.json",
			expectedKind:      "",
			expectedNamespace: "",
			expectedName:      "payload.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kind, namespace, name := formatResourceDisplay(tc.resourceKey)
			assert.Equal(t, tc.expectedKind, kind, "kind mismatch")
			assert.Equal(t, tc.expectedNamespace, namespace, "namespace mismatch")
			assert.Equal(t, tc.expectedName, name, "name mismatch")
		})
	}
}

// TestCreateExcludedRowResourceFormatting verifies that createExcludedRow displays the resource
// with namespace, kind, and name properly isolated.
func TestCreateExcludedRowResourceFormatting(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})
	color.Init(true)

	testResult := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
		},
	}

	t.Run("namespaced resource", func(t *testing.T) {
		row := createExcludedRow(testResult, 1, "apps/v1,Deployment,staging,my-deploy", false)
		assert.Equal(t, "staging/apps/v1/Deployment/my-deploy", row.Resource)
	})

	t.Run("cluster-scoped resource", func(t *testing.T) {
		row := createExcludedRow(testResult, 2, "rbac.authorization.k8s.io/v1,ClusterRole,,cluster-admin", false)
		assert.Equal(t, "rbac.authorization.k8s.io/v1/ClusterRole/cluster-admin", row.Resource)
	})
}

// TestCreateRowsAccordingToResultsResourceFormatting verifies createRowsAccordingToResults
// formats the resource with namespace, kind, and name correctly.
func TestCreateRowsAccordingToResultsResourceFormatting(t *testing.T) {
	origNoColor := fatihcolor.NoColor
	t.Cleanup(func() {
		fatihcolor.NoColor = origNoColor
		color.Init(origNoColor)
	})
	color.Init(true)

	testResult := v1alpha1.TestResult{
		TestResultBase: v1alpha1.TestResultBase{
			Policy: "test-policy",
			Rule:   "test-rule",
		},
	}

	t.Run("namespaced pod from resource key", func(t *testing.T) {
		rc := &resultCounts{}
		counter := 1
		rows := createRowsAccordingToResults(testResult, rc, &counter, "check-pod", true, "Ok", "Pass", "v1,Pod,default,my-pod")
		assert.Len(t, rows, 1)
		assert.Equal(t, "default/v1/Pod/my-pod", rows[0].Resource)
	})

	t.Run("cluster-scoped resource from resource key", func(t *testing.T) {
		rc := &resultCounts{}
		counter := 1
		rows := createRowsAccordingToResults(testResult, rc, &counter, "check-cr", true, "Ok", "Pass", "rbac.authorization.k8s.io/v1,ClusterRole,,admin")
		assert.Len(t, rows, 1)
		assert.Equal(t, "rbac.authorization.k8s.io/v1/ClusterRole/admin", rows[0].Resource)
	})
}

// TestResourceDisplayPathRoundTrip asserts that resourceDisplayPath emits correctly structured
// paths and that formatResourceDisplay round-trips them preserving kind, namespace, and name.
func TestResourceDisplayPathRoundTrip(t *testing.T) {
	testCases := []struct {
		name              string
		canonicalKey      string
		expectedPath      string
		expectedKind      string
		expectedNamespace string
		expectedName      string
	}{
		{
			name:              "namespaced core pod",
			canonicalKey:      "v1,Pod,default,my-pod",
			expectedPath:      "default/v1/Pod/my-pod",
			expectedKind:      "v1/Pod",
			expectedNamespace: "default",
			expectedName:      "my-pod",
		},
		{
			name:              "namespaced group deployment",
			canonicalKey:      "apps/v1,Deployment,staging,my-deploy",
			expectedPath:      "staging/apps/v1/Deployment/my-deploy",
			expectedKind:      "apps/v1/Deployment",
			expectedNamespace: "staging",
			expectedName:      "my-deploy",
		},
		{
			name:              "cluster-scoped group clusterrole",
			canonicalKey:      "rbac.authorization.k8s.io/v1,ClusterRole,,admin",
			expectedPath:      "rbac.authorization.k8s.io/v1/ClusterRole/admin",
			expectedKind:      "rbac.authorization.k8s.io/v1/ClusterRole",
			expectedNamespace: "",
			expectedName:      "admin",
		},
		{
			name:              "cluster-scoped core clusterrole",
			canonicalKey:      "v1,ClusterRole,,cluster-admin",
			expectedPath:      "v1/ClusterRole/cluster-admin",
			expectedKind:      "v1/ClusterRole",
			expectedNamespace: "",
			expectedName:      "cluster-admin",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := resourceDisplayPath(tc.canonicalKey)
			assert.Equal(t, tc.expectedPath, path, "display path mismatch")

			kind, namespace, name := formatResourceDisplay(path)
			assert.Equal(t, tc.expectedKind, kind, "kind round-trip mismatch")
			assert.Equal(t, tc.expectedNamespace, namespace, "namespace round-trip mismatch")
			assert.Equal(t, tc.expectedName, name, "name round-trip mismatch")
		})
	}
}
