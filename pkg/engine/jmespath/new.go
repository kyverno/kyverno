package jmespath

import (
	gojmespath "github.com/jmespath/go-jmespath"
)

func newJMESPath(query string) (*gojmespath.JMESPath, error) {
	jp, err := gojmespath.Compile(query)
	if err != nil {
		return nil, err
	}
	for _, function := range GetFunctions() {
		jp.Register(function.FunctionEntry)
	}
	return jp, nil
}
