package resource

import (
	"path/filepath"
	"testing"

	"gotest.tools/assert"
)

func TestRemoveDuplicates(t *testing.T) {
	type TestCase struct {
		testFile           string
		expectedResources  int
		expectedDuplicates int
	}
	baseTestDir := "../_testdata/resources"
	tests := []*TestCase{
		{

			testFile:           "with-duplicate.yaml",
			expectedResources:  6,
			expectedDuplicates: 1,
		},
		{
			testFile:           "all-unique.yaml",
			expectedResources:  6,
			expectedDuplicates: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.testFile, func(t *testing.T) {
			fileBytes, err := GetFileBytes(filepath.Join(baseTestDir, tt.testFile))
			assert.NilError(t, err)
			resources, err := GetUnstructuredResources(fileBytes)
			assert.NilError(t, err)

			uniques, duplicates := RemoveDuplicates(resources)
			assert.Equal(t, len(uniques), tt.expectedResources, "Did not get the expected number of resources for test %s", tt.testFile)
			assert.Equal(t, len(duplicates), tt.expectedDuplicates, "Did not get the expected number of duplicates for test %s", tt.testFile)
		})
	}
}
