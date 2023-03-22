package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/test/api"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"gotest.tools/assert"
	"sigs.k8s.io/yaml"
)

func Test_selectResourcesForCheck(t *testing.T) {

	type TestCase struct {
		testFile           string
		expectedResources  int
		expectedDuplicates int
		expectedUnused     int
	}
	baseTestDir := "../../../../test/cli/test-unit/selectResourcesForCheck/"
	testcases := []*TestCase{
		{

			testFile:           "kyverno-test-duplicated-with-resource.yaml",
			expectedResources:  3,
			expectedDuplicates: 1,
			expectedUnused:     3,
		},
		{
			testFile:           "kyverno-test-duplicated-with-resources.yaml",
			expectedResources:  3,
			expectedDuplicates: 1,
			expectedUnused:     3,
		},
		{
			testFile:           "kyverno-test-uniq-with-resource.yaml",
			expectedResources:  3,
			expectedDuplicates: 0,
			expectedUnused:     3,
		},
		{
			testFile:           "kyverno-test-uniq-with-resources.yaml",
			expectedResources:  3,
			expectedDuplicates: 0,
			expectedUnused:     3,
		},
	}
	fs := memfs.New()
	for _, tc := range testcases {

		// read test spec
		values := &api.Test{}
		testBytes, err := os.ReadFile(filepath.Join(baseTestDir, tc.testFile))
		assert.NilError(t, err)
		err = yaml.Unmarshal(testBytes, values)
		assert.NilError(t, err)

		// read policies
		policies, err := common.GetPoliciesFromPaths(
			fs,
			[]string{filepath.Join(baseTestDir, values.Policies[0])},
			false,
			filepath.Join(baseTestDir, values.Resources[0]),
		)
		assert.NilError(t, err)

		// read resources
		resources, err := common.GetResourceAccordingToResourcePath(
			fs,
			[]string{filepath.Join(baseTestDir, values.Resources[0])},
			false,
			policies,
			nil,
			"",
			false,
			false,
			filepath.Join(baseTestDir, values.Policies[0]),
		)
		assert.NilError(t, err)

		selected, duplicates, unused := selectResourcesForCheckInternal(resources, values)
		assert.Equal(t, len(selected), tc.expectedResources,
			"Did not get the expected number of resources for test %s", tc.testFile)
		assert.Equal(t, duplicates, tc.expectedDuplicates,
			"Did not get the expected number of duplicates for test %s", tc.testFile)
		assert.Equal(t, unused, tc.expectedUnused,
			"Did not get the expected number of unused resources for test %s", tc.testFile)
	}
}
