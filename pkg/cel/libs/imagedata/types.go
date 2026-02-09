package imagedata

import (
	"github.com/google/cel-go/common/types"
)

var ContextType = types.NewOpaqueType("imagedata.Context")

type ContextInterface interface {
	GetImageData(string) (map[string]any, error)
}

type Context struct {
	ContextInterface
}
