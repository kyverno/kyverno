package testutils

import (
	"fmt"
	"testing"
)

func TestUtils(t *testing.T) {
	file := "/Users/shiv/nirmata/code/go/src/github.com/nirmata/kyverno/examples/mutate/patches"
	ts := LoadTestSuite(t, file)
	// policy application logic
	tp := &testPolicy{}
	ts.setApplyTest(tp)
	// run the tests for each test bundle
	ts.runTests()
	if ts != nil {
		fmt.Println("Done building the test bundles")
	}
	// run the tests against the policy engine
}
