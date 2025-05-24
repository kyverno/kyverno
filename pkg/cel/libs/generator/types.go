package generator

import (
	"github.com/google/cel-go/common/types"
)

var ContextType = types.NewOpaqueType("generator.Context")

type ContextInterface interface {
	GenerateResources(namespace string, dataList []map[string]any) error
}

type Context struct {
	ContextInterface
}
