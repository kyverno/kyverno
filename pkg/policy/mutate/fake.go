package mutate

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/policy/mutate/fake"
)

// FakeGenerate provides implementation for generate rule processing
// with mocks/fakes for cluster interactions
type FakeMutate struct {
	Mutate
}

// NewFakeGenerate returns a new instance of generatecheck that uses
// fake/mock implementation for operation access(always returns true)
func NewFakeMutate(mutation kyvernov1.Mutation) *FakeMutate {
	m := FakeMutate{}
	m.mutation = mutation
	m.authChecker = fake.NewFakeAuth()
	m.user = "Kyverno"
	return &m
}
