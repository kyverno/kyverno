package testutils

import (
	"testing"

	"github.com/golang/glog"
)

// func TestExamples(t *testing.T) {
// 	folders := []string{
// 		"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples",
// 	}
// 	testrunner(t, folders)
// }

func TestGenerate(t *testing.T) {
	folders := []string{
		"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/generate",
	}
	testrunner(t, folders)
}

func TestMutateOverlay(t *testing.T) {
	folders := []string{
		"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/mutate/overlay",
	}
	testrunner(t, folders)
}

func TestMutatePatches(t *testing.T) {
	folders := []string{
		"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/mutate/patches",
	}
	testrunner(t, folders)
}

func testrunner(t *testing.T, folders []string) {
	for _, folder := range folders {
		runTest(t, folder)
	}
}

func runTest(t *testing.T, path string) {
	// Load test suites at specified path
	ts := LoadTestSuite(t, path)
	// policy application logic
	tp := &testPolicy{}
	ts.setApplyTest(tp)
	// run the tests for each test bundle
	ts.runTests()
	if ts != nil {
		glog.Infof("Done running the test at %s", path)
	}
}
