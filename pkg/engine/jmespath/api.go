package jmespath

import (
	gojmespath "github.com/jmespath-community/go-jmespath"
	"github.com/jmespath-community/go-jmespath/pkg/interpreter"
	"github.com/jmespath-community/go-jmespath/pkg/parsing"
)

type jmesPath struct {
	node parsing.ASTNode
}

func newCommunityJMESPath(node parsing.ASTNode) gojmespath.JMESPath {
	return jmesPath{
		node: node,
	}
}

// Compile parses a JMESPath expression and returns, if successful, a JMESPath
// object that can be used to match against data.
func Compile(expression string) (gojmespath.JMESPath, error) {
	parser := parsing.NewParser()
	ast, err := parser.Parse(expression)
	if err != nil {
		return nil, err
	}
	return newCommunityJMESPath(ast), nil
}

// Search evaluates a JMESPath expression against input data and returns the result.
func (jp jmesPath) Search(data interface{}, opts ...interpreter.Option) (interface{}, error) {
	intr := NewInterpreter(data, nil)
	return intr.Execute(jp.node, data, opts...)
}

// Search evaluates a JMESPath expression against input data and returns the result.
func Search(expression string, data interface{}, opts ...interpreter.Option) (interface{}, error) {
	compiled, err := Compile(expression)
	if err != nil {
		return nil, err
	}
	return compiled.Search(data, opts...)
}
