package testutils

import (
	"testing"

	"github.com/golang/glog"
)

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

func TestExamples(t *testing.T) {
	// folders := []string{
	// 	"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/mutate/patches",
	// 	"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/mutate/overlay",
	// 	"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/cli",
	// }
	folders := []string{
		"/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples",
	}
	for _, folder := range folders {
		runTest(t, folder)
	}
}
