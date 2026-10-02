package test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A namespaced policy only applies to resources in its own namespace, which is how the
// deleting controller and the image validating webhooks scope it in a cluster.
func TestRunTest_NamespacedPolicyScope(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	rootDir := filepath.Join(wd, "..", "..", "..", "..", "..")

	testcases := map[string]struct {
		testDir         string
		policyNamespace string
	}{
		"deleting policy": {
			testDir:         filepath.Join(rootDir, "test", "cli", "test-deleting-policy", "namespaced-scope"),
			policyNamespace: "team-a",
		},
		"image validating policy": {
			testDir:         filepath.Join(rootDir, "test", "cli", "test-image-validating-policy", "namespaced-scope"),
			policyNamespace: "default",
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			testFile := filepath.Join(tc.testDir, "kyverno-test.yaml")
			testCases := test.LoadTest(nil, testFile)
			require.Len(t, testCases, 1, "Expected exactly one test case in %s", testFile)

			out := &bytes.Buffer{}
			testResponse, err := runTest(context.TODO(), out, testCases[0], false)
			require.NoError(t, err, "runTest %s: %s", name, out.String())

			evaluated := map[string]int{}
			for _, responses := range testResponse.Trigger {
				for _, response := range responses {
					if len(response.PolicyResponse.Rules) > 0 {
						evaluated[response.Resource.GetNamespace()]++
					}
				}
			}
			assert.Greater(t, evaluated[tc.policyNamespace], 0, "the policy should be evaluated in its own namespace")
			for namespace := range evaluated {
				assert.Equal(t, tc.policyNamespace, namespace, "the policy should not be evaluated in another namespace")
			}
		})
	}
}
