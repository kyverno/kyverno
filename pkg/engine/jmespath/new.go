package jmespath

import (
	gojmespath "github.com/jmespath-community/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

// newImplementation just returns our no‑frills engine.
func newImplementation(_ config.Configuration) Interface {
	return implementation{}
}

// newJMESPath compiles a JMESPath expression into a Query.
func newJMESPath(expression string) (Query, error) {
	return gojmespath.Compile(expression)
}

// newExecution runs a one‑off query.
func newExecution(expression string, data interface{}) (interface{}, error) {
	return gojmespath.Search(expression, data)
}
