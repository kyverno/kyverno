package store

import "github.com/kyverno/kyverno/pkg/engine/jmespath"

type Projection struct {
	Name string
	JP   jmespath.Query
}

type Entry interface {
	Get(projection string) (any, error)
	Stop()
}
