package context

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ContextType = types.NewOpaqueType("context.Context")

type ContextInterface interface {
	GetConfigMap(string, string) (unstructured.Unstructured, error)
	GetGlobalReference(string) (any, error)
	GetImageData(string) (any, error)
}

type Context struct {
	ContextInterface
}
