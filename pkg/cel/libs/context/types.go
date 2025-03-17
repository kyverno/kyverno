package context

import (
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiservercel "k8s.io/apiserver/pkg/cel"
)

var (
	ContextType   = types.NewOpaqueType("context.Context")
	configMapType = BuildConfigMapType()
	imageDataType = BuildImageDataType()
)

type ContextInterface interface {
	GetConfigMap(string, string) (*unstructured.Unstructured, error)
	GetImageData(string) (map[string]interface{}, error)
	ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error)
	GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
}

type Context struct {
	ContextInterface
}

func BuildTypeMetaType() *apiservercel.DeclType {
	return apiservercel.NewObjectType("kubernetes.TypeMeta", fields(
		field("apiVersion", apiservercel.StringType, true),
		field("kind", apiservercel.StringType, true),
	))
}

func BuildObjectMetaType() *apiservercel.DeclType {
	return apiservercel.NewObjectType("kubernetes.ObjectMeta", fields(
		field("name", apiservercel.StringType, true),
		field("generateName", apiservercel.StringType, true),
		field("namespace", apiservercel.StringType, true),
		field("labels", apiservercel.NewMapType(apiservercel.StringType, apiservercel.StringType, -1), true),
		field("annotations", apiservercel.NewMapType(apiservercel.StringType, apiservercel.StringType, -1), true),
		field("UID", apiservercel.StringType, true),
		field("creationTimestamp", apiservercel.TimestampType, true),
		field("deletionGracePeriodSeconds", apiservercel.IntType, true),
		field("deletionTimestamp", apiservercel.TimestampType, true),
		field("generation", apiservercel.IntType, true),
		field("resourceVersion", apiservercel.StringType, true),
		field("finalizers", apiservercel.NewListType(apiservercel.StringType, -1), true),
	))
}

func BuildConfigMapType() *apiservercel.DeclType {
	typeMeta := BuildTypeMetaType()
	objectMeta := BuildObjectMetaType()
	f := make([]*apiservercel.DeclField, 0, len(typeMeta.Fields))
	for _, field := range typeMeta.Fields {
		f = append(f, field)
	}
	f = append(f,
		field("metadata", objectMeta, true),
		field("data", apiservercel.NewMapType(apiservercel.StringType, apiservercel.StringType, -1), true),
	)
	return apiservercel.NewObjectType("kubernetes.ConfigMap", fields(f...))
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
