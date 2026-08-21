package jmespath

import (
	"github.com/dgraph-io/ristretto/v2"
	gojmespath "github.com/kyverno/go-jmespath"
	"github.com/kyverno/kyverno/pkg/config"
)

const defaultQueryCacheSize = 1024

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

func (i *implementation) getQuery(query string) (*QueryProxy, error) {
	if i.cache != nil {
		if proxy, ok := i.cache.Get(query); ok {
			return proxy, nil
		}
	}
	proxy, err := newJMESPath(query, i.functionCaller)
	if err != nil {
		return nil, err
	}
	if i.cache != nil {
		i.cache.Set(query, proxy, 1)
		i.cache.Wait()
	}
	return proxy, nil
}

func newImplementation(configuration config.Configuration) Interface {
	functionCaller := gojmespath.NewFunctionCaller()
	functions := GetFunctions(configuration)
	for _, f := range functions {
		functionCaller.Register(f.FunctionEntry)
	}

	cache, err := ristretto.NewCache(&ristretto.Config[string, *QueryProxy]{
		MaxCost:     defaultQueryCacheSize,
		NumCounters: 10 * defaultQueryCacheSize,
		BufferItems: 64,
	})
	if err != nil {
		return &implementation{functionCaller: functionCaller}
	}
	return &implementation{functionCaller: functionCaller, cache: cache}
}
