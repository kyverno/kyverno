package jmespath

import (
	"github.com/jmespath-community/go-jmespath"
	"github.com/jmespath-community/go-jmespath/pkg/functions"
)

func New(query string) (jmespath.JMESPath, error) {
	var funcs []functions.FunctionEntry
	for _, f := range GetFunctions() {
		funcs = append(funcs, f.FunctionEntry)
	}
	jp, err := jmespath.Compile(query, funcs...)
	if err != nil {
		return nil, err
	}
	return jp, nil
}
