package resource

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
	"k8s.io/apiserver/pkg/cel/openapi"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

const TypeName = "self"

// https://pkg.go.dev/k8s.io/apiserver@v0.32.0/pkg/cel/openapi/resolver#ClientDiscoveryResolver
type SchemaClient interface {
	ResolveSchema(gvk schema.GroupVersionKind) (*spec.Schema, error)
}

type OpenAPITypeResolver struct {
	client SchemaClient
	c      *resolver.ClientDiscoveryResolver
}

func (o *OpenAPITypeResolver) GetDeclProvier(gvk schema.GroupVersionKind) (*cel.DeclTypeProvider, error) {
	spec, err := o.client.ResolveSchema(gvk)
	if err != nil {
		return nil, err
	}

	schema := common.SchemaDeclType(&openapi.Schema{Schema: spec}, true)

	return cel.NewDeclTypeProvider(schema.MaybeAssignTypeName(TypeName)), nil
}

func NewOpenAPITypeResolver(client SchemaClient) *OpenAPITypeResolver {
	return &OpenAPITypeResolver{client: client}
}
