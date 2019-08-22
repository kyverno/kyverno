package testrunner

import "testing"

func TestCLI(t *testing.T) {
	//https://github.com/nirmata/kyverno/issues/301
	t.Skip("skipping testrunner as this needs a re-design")
	runner(t, "/test/scenarios/cli")
}
