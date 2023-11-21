package jmespath

import (
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

func newJMESPath(fCall *gojmespath.FunctionCaller, query string) (*gojmespath.JMESPath, error) {
	parser := gojmespath.NewParser()
	ast, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}

	intr := gojmespath.NewInterpreterFromFunctionCaller(fCall)

	return gojmespath.NewJMESPath(ast, intr), nil
}

func newImplementation(configuration config.Configuration) Interface {
	functionCaller := gojmespath.NewFunctionCaller()
	functions := GetFunctions(configuration)
	for _, f := range functions {
		functionCaller.Register(f.FunctionEntry)
	}

	return implementation{
		functionCaller,
	}
}

func newExecution(fCall *gojmespath.FunctionCaller, query string, data interface{}) (interface{}, error) {
	parser := gojmespath.NewParser()
	ast, err := parser.Parse(query)
	if err != nil {
		return nil, err
	}

	intr := gojmespath.NewInterpreterFromFunctionCaller(fCall)

	return intr.Execute(ast, data)
}
