package framework

import (
	"os"
	"testing"

	"github.com/kyverno/kyverno/test/e2e/framework/client"
	"github.com/kyverno/kyverno/test/e2e/framework/step"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func Setup(t *testing.T) {
	t.Helper()
	gomega.RegisterTestingT(t)
	if os.Getenv("E2E") == "" {
		t.Skip("Skipping E2E Test")
	}
}

func RunTest(t *testing.T, steps ...step.Step) {
	t.Helper()
	ginkgo.By("Creating client ...")
	client := client.New(t)
	for _, step := range steps {
		step(client)
	}
	ginkgo.By("Cleaning up ...")
}

func RunSubTest(t *testing.T, name string, steps ...step.Step) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		RunTest(t, steps...)
	})
}
