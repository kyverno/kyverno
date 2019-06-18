package testrunner

import "testing"

func TestExamples(t *testing.T) {
	runner(t, "/test/scenarios")
}
