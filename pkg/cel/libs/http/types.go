package http

import (
	"github.com/google/cel-go/common/types"
)

var ContextType = types.NewOpaqueType("http.Context")

type ContextInterface interface {
	Get(url string, headers map[string]string) (any, error)
	Post(url string, data any, headers map[string]string) (any, error)
	Client(caBundle string) (ContextInterface, error)
}

type Context struct {
	ContextInterface
}
