package jmespath

import (
	gojmespath "github.com/kyverno/go-community-jmespath"
	"github.com/kyverno/go-community-jmespath/pkg/functions"
	"github.com/kyverno/go-community-jmespath/pkg/interpreter"
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
	jmesPath, err := gojmespath.Compile(query)
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
	entries := make([]functions.FunctionEntry, 0, len(list))
	for _, f := range list {
		entries = append(entries, f.FunctionEntry)
	}

	return implementation{
		functionCaller: interpreter.NewFunctionCaller(entries...),
	}
}

func newExecution(fCall interpreter.FunctionCaller, query string, data interface{}) (interface{}, error) {
	return gojmespath.Search(query, data, gojmespath.WithFunctionCaller(fCall))
}
