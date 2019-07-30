package testrunner

import "testing"

func TestCLI(t *testing.T) {
	runner(t, "/test/scenarios/cli")
}
