package jmespath

import (
	gojmespath "github.com/jmespath-community/go-jmespath"
	"github.com/jmespath-community/go-jmespath/pkg/functions"
	"github.com/jmespath-community/go-jmespath/pkg/interpreter"
	"github.com/kyverno/kyverno/pkg/config"
)

type QueryProxy struct {
	jmesPath       gojmespath.JMESPath
	functionCaller interpreter.FunctionCaller
}

func (q *QueryProxy) Search(data interface{}) (interface{}, error) {
	return q.jmesPath.Search(data, gojmespath.WithFunctionCaller(q.functionCaller))
}

func newJMESPath(query string, functionCaller interpreter.FunctionCaller) (*QueryProxy, error) {
	jmesPath, err := Compile(query)
	if err != nil {
		return nil, err
	}
	return &QueryProxy{
		jmesPath,
		functionCaller,
	}, nil
}

func newImplementation(configuration config.Configuration) Interface {
	list := GetFunctions(configuration)
	entries := functions.GetDefaultFunctions()
	for _, f := range list {
		entries = append(entries, f.FunctionEntry)
	}

	return implementation{
		functionCaller: interpreter.NewFunctionCaller(entries...),
	}
}

func newExecution(fCall interpreter.FunctionCaller, query string, data interface{}) (interface{}, error) {
	return Search(query, data, gojmespath.WithFunctionCaller(fCall))
}
