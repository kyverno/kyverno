package testrunner

import "testing"

func TestMutate(t *testing.T) {
	runner(t, "/test/scenarios/mutate")
}

func TestCLI(t *testing.T) {
	runner(t, "/test/scenarios/cli")
}

func TestGenerate(t *testing.T) {
	runner(t, "/test/scenarios/generate")
}
