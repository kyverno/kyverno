package jmespath

import (
	"github.com/dgraph-io/ristretto/v2"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

type Query interface {
	Search(interface{}) (interface{}, error)
}

type Interface interface {
	Query(string) (Query, error)
	Search(string, interface{}) (interface{}, error)
}

type implementation struct {
	functionCaller *gojmespath.FunctionCaller
	cache          *ristretto.Cache[string, *QueryProxy]
}

func New(configuration config.Configuration) Interface {
	return newImplementation(configuration)
}

func (i *implementation) Query(query string) (Query, error) {
	return i.getQuery(query)
}

func (i *implementation) Search(query string, data interface{}) (interface{}, error) {
	proxy, err := i.getQuery(query)
	if err != nil {
		return nil, err
	}
	return proxy.Search(data)
}
