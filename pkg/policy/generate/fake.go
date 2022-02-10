package generate

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policy/generate/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//FakeGenerate provides implementation for generate rule processing
// with mocks/fakes for cluster interactions
type FakeGenerate struct {
	Generate
}

//NewFakeGenerate returns a new instance of generatecheck that uses
// fake/mock implementation for operation access(always returns true)
func NewFakeGenerate(rule kyverno.Generation) *FakeGenerate {
	g := FakeGenerate{}
	g.rule = rule
	g.authCheck = fake.NewFakeAuth()
	g.log = log.Log
	return &g
}
