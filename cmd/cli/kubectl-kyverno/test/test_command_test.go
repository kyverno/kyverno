package test

import (
	"log"
	"testing"

	"gotest.tools/assert"
)

func Test_Test(t *testing.T) {
	// dirPath, fileName, gitBranch, testCase, failOnly
	type TestCase struct {
		DirPath   []string
		FileName  string
		GitBranch string
		// TestCase  resultCounts
		TestCase            string
		FailOnly            bool
		ExpectedResultCount resultCounts
	}

	testcases := []TestCase{
		// {
		// 	DirPath: []string{"../../../../test/cli/test/wildcard_mutate_targets"},
		// 	// DirPath: []string{"../../../../test/cli/test/wildcard_mutate"},

		// 	FileName:  "kyverno-test.yaml",
		// 	GitBranch: "",
		// 	TestCase:  "",
		// 	FailOnly:  true,
		// 	ExpectedResultCount: resultCounts{
		// 		Skip: 0,
		// 		Pass: 2,
		// 		Fail: 0,
		// 	},
		// },
		// {
		// 	DirPath: []string{"../../../../test/cli/test-generate/sync-spot-controller-data-TODO"},
		// 	// DirPath: []string{"../../../../test/cli/test/wildcard_mutate"},

		// 	FileName:  "kyverno-test.yaml",
		// 	GitBranch: "",
		// 	TestCase:  "",
		// 	FailOnly:  true,
		// 	ExpectedResultCount: resultCounts{
		// 		Skip: 0,
		// 		Pass: 2,
		// 		Fail: 0,
		// 	},
		// },
		{
			DirPath: []string{"../../../../test/cli/test/pod-default-resources-based-on-ports"},
			// DirPath: []string{"../../../../test/cli/test/wildcard_mutate"},

			FileName:  "kyverno-test.yaml",
			GitBranch: "",
			TestCase:  "",
			FailOnly:  true,
			ExpectedResultCount: resultCounts{
				Skip: 0,
				Pass: 2,
				Fail: 0,
			},
		},
	}

	for _, tc := range testcases {
		resultCount, err := testCommandExecute(tc.DirPath, tc.FileName, tc.GitBranch, tc.TestCase, tc.FailOnly, false)
		if err != nil {
			log.Println("a directory is required")
			return
		}
		expectedResultCount := &tc.ExpectedResultCount
		givenResultCount := resultCount
		assert.Equal(t, givenResultCount.Fail, expectedResultCount.Fail)
		assert.Equal(t, givenResultCount.Pass, expectedResultCount.Pass)
		assert.Equal(t, givenResultCount.Skip, expectedResultCount.Skip)

	}
}
