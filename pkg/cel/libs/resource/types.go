package resource

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

var (
	ContextType   = types.NewOpaqueType("resource.Context")
	imageDataType = BuildImageDataType()
)

type ContextInterface interface {
	GetImageData(string) (map[string]interface{}, error)
	ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error)
	GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
}

type Context struct {
	ContextInterface
}

func BuildImageDataType() *apiservercel.DeclType {
	f := make([]*apiservercel.DeclField, 0)
	f = append(f,
		field("image", apiservercel.StringType, true),
		field("resolvedImage", apiservercel.StringType, true),
		field("registry", apiservercel.StringType, true),
		field("repository", apiservercel.StringType, true),
		field("tag", apiservercel.StringType, false),
		field("digest", apiservercel.StringType, false),
		field("imageIndex", apiservercel.DynType, false),
		field("manifest", apiservercel.DynType, true),
		field("config", apiservercel.DynType, true),
	)
	return apiservercel.NewObjectType("imageData", fields(f...))
}

func field(name string, declType *apiservercel.DeclType, required bool) *apiservercel.DeclField {
	return apiservercel.NewDeclField(name, declType, required, nil, nil)
}

func fields(fields ...*apiservercel.DeclField) map[string]*apiservercel.DeclField {
	result := make(map[string]*apiservercel.DeclField, len(fields))
	for _, f := range fields {
		result[f.Name] = f
	}
	return result
}
