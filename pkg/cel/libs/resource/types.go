package resource

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var ContextType = types.NewOpaqueType("resource.Context")

var GVRType = types.NewOpaqueType("schema.GroupVersionResource")

type ContextInterface interface {
	ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error)
	GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
	PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error)
	ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error)
}

type Context struct {
	ContextInterface
}
