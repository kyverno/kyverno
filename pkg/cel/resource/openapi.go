package resource

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/openapi"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/client-go/discovery"
)

type OpenAPITypeResolver struct {
	client *resolver.ClientDiscoveryResolver
}

func (o *OpenAPITypeResolver) GetDecl(gvk schema.GroupVersionKind) (*cel.DeclType, error) {
	spec, err := o.client.ResolveSchema(gvk)
	if err != nil {
		return nil, err
	}

	return common.SchemaDeclType(&openapi.Schema{Schema: spec}, true), nil
}

func NewOpenAPITypeResolver(d discovery.DiscoveryInterface) *OpenAPITypeResolver {
	return &OpenAPITypeResolver{
		client: &resolver.ClientDiscoveryResolver{Discovery: d},
	}
}
