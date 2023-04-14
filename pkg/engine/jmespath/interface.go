package jmespath

import "github.com/kyverno/kyverno/pkg/config"

type Query interface {
	Search(interface{}) (interface{}, error)
}

type Interface interface {
	Query(string) (Query, error)
	Search(string, interface{}) (interface{}, error)
}

type implementation struct {
	configuration config.Configuration
}

func New(configuration config.Configuration) Interface {
	return implementation{
		configuration: configuration,
	}
}

func (i implementation) Query(query string) (Query, error) {
	return newJMESPath(i.configuration, query)
}

func (i implementation) Search(query string, data interface{}) (interface{}, error) {
	if query, err := i.Query(query); err != nil {
		return nil, err
	} else {
		return query.Search(data)
	}
}
