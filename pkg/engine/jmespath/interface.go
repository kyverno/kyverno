package jmespath

import (
	gojmespath "github.com/jmespath-community/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

type Query interface {
	Search(data interface{}, opts ...gojmespath.Option) (interface{}, error)
}

type Interface interface {
	Query(string) (Query, error)
	Search(string, interface{}) (interface{}, error)
}

type implementation struct {
	//functionCaller *gojmespath.FunctionCaller
}

func New(_ config.Configuration) Interface {
	return implementation{}
}

func (i implementation) Query(expression string) (Query, error) {
	return gojmespath.Compile(expression)
}
func (i implementation) Search(query string, data interface{}) (interface{}, error) {
	return gojmespath.Search(query, data)
}
