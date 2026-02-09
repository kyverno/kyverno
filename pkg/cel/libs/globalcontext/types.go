package globalcontext

import (
	"github.com/google/cel-go/common/types"
)

var ContextType = types.NewOpaqueType("globalcontext.Context")

type ContextInterface interface {
	GetGlobalReference(string, string) (any, error)
}

type Context struct {
	ContextInterface
}
