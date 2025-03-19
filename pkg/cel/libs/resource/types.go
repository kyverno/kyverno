package resource

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var ContextType = types.NewOpaqueType("resource.Context")

type ContextInterface interface {
	ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error)
	GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
}

type Context struct {
	ContextInterface
}
