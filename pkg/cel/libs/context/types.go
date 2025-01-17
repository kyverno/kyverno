package context

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ContextType = types.NewObjectType("context.Context")

type Context interface {
	GetConfigMap(string, string) (unstructured.Unstructured, error)
}
