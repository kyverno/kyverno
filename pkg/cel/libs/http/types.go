package http

import (
	"github.com/google/cel-go/common/types"
)

var ContextType = types.NewOpaqueType("http.Context")

type ContextInterface interface {
	Get(url string, headers map[string]string) (map[string]any, error)
	Post(url string, data map[string]any, headers map[string]string) (map[string]any, error)
	Client(caBundle string) (ContextInterface, error)
}

type Context struct {
	ContextInterface
}
