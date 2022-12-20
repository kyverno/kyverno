package generate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/policy/generate/fake"
)

// FakeGenerate provides implementation for generate rule processing
// with mocks/fakes for cluster interactions
type FakeGenerate struct {
	Generate
}

// NewFakeGenerate returns a new instance of generatecheck that uses
// fake/mock implementation for operation access(always returns true)
func NewFakeGenerate(rule kyvernov1.Generation) *FakeGenerate {
	g := FakeGenerate{}
	g.rule = rule
	g.authCheck = fake.NewFakeAuth()
	g.log = logging.GlobalLogger()
	return &g
}
