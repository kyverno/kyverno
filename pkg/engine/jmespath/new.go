package jmespath

import (
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

func newJMESPath(intr gojmespath.Interpreter, query string) (*gojmespath.JMESPath, error) {
	parser := gojmespath.NewParser()
	ast, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}

	return gojmespath.NewJMESPath(ast, intr), nil
}

func newImplementation(configuration config.Configuration) Interface {
	i := gojmespath.NewInterpreter()
	functions := GetFunctions(configuration)
	for _, f := range functions {
		i.Register(f.FunctionEntry)
	}

	return implementation{
		interpreter: i,
	}
}

func newExecution(intr gojmespath.Interpreter, query string, data interface{}) (interface{}, error) {
	parser := gojmespath.NewParser()
	ast, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}

	return intr.Execute(ast, data)
}
