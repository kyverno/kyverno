package jmespath

import (
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

func newJMESPath(configuration config.Configuration, query string) (*gojmespath.JMESPath, error) {
	jp, err := gojmespath.Compile(query)
	if err != nil {
		return nil, err
	}
	for _, function := range GetFunctions(configuration) {
		jp.Register(function.FunctionEntry)
	}
	return jp, nil
}
