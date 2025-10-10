package resource

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"
)

var ContextType = types.NewOpaqueType("resource.Context")

var GVRType = types.NewOpaqueType("schema.GroupVersionResource")

func VersionedLibrary() environment.VersionedOptions {
	return environment.VersionedOptions{
		IntroducedVersion: version.MajorMinor(1, 0),
		EnvOptions: []cel.EnvOption{
			Lib(),
		},
	}
}

type ContextInterface interface {
	ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error)
	GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error)
	PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error)
	ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error)
}

type Context struct {
	ContextInterface
}
