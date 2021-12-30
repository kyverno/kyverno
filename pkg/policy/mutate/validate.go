package mutate

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	// rule to hold 'mutate' rule specifications
	rule kyverno.Mutation
}

//NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(rule kyverno.Mutation) *Mutate {
	m := Mutate{
		rule: rule,
	}
	return &m
}

//Validate validates the 'mutate' rule
func (m *Mutate) Validate() (string, error) {

	return "", nil
}
