package jmespath

import (
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

type QueryProxy struct {
	jmesPath       *gojmespath.JMESPath
	functionCaller *gojmespath.FunctionCaller
}

func (q *QueryProxy) Search(data interface{}) (interface{}, error) {
	return q.jmesPath.Search(data, gojmespath.WithFunctionCaller(q.functionCaller))
}

func newJMESPath(query string, functionCaller *gojmespath.FunctionCaller) (*QueryProxy, error) {
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
	return gojmespath.Search(query, data, gojmespath.WithFunctionCaller(fCall))
}
