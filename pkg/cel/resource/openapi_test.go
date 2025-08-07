package resource_test

import (
	"errors"
	"testing"

	"github.com/google/cel-go/cel"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/resource"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kube-openapi/pkg/validation/spec"
)

func testSchema() *spec.Schema {
	// Manual construction of a schema with the following definition:
	//
	// schema:
	//   type: object
	//   metadata:
	//     custom_type: "CustomObject"
	//   required:
	//     - name
	//     - value
	//   properties:
	//     name:
	//       type: string
	//     nested:
	//       type: object
	//       properties:
	//         subname:
	//           type: string
	//         flags:
	//           type: object
	//           additionalProperties:
	//             type: boolean
	//         dates:
	//           type: array
	//           items:
	//             type: string
	//             format: date-time
	//      metadata:
	//        type: object
	//        additionalProperties:
	//          type: object
	//          properties:
	//            key:
	//              type: string
	//            values:
	//              type: array
	//              items: string
	//     value:
	//       type: integer
	//       format: int64
	//       default: 1
	//       enum: [1,2,3]
	return &spec.Schema{
		SchemaProps: spec.SchemaProps{
			Type: []string{"object"},
			Properties: map[string]spec.Schema{
				"name": *spec.StringProperty(),
				"value": {SchemaProps: spec.SchemaProps{
					Type:    []string{"integer"},
					Default: int64(1),
					Format:  "int64",
					Enum:    []any{1, 2, 3},
				}},
				"nested": {SchemaProps: spec.SchemaProps{
					Type: []string{"object"},
					Properties: map[string]spec.Schema{
						"subname": *spec.StringProperty(),
						"flags": {SchemaProps: spec.SchemaProps{
							Type: []string{"object"},
							AdditionalProperties: &spec.SchemaOrBool{
								Schema: spec.BooleanProperty(),
							},
						}},
						"dates": {SchemaProps: spec.SchemaProps{
							Type: []string{"array"},
							Items: &spec.SchemaOrArray{Schema: &spec.Schema{
								SchemaProps: spec.SchemaProps{
									Type:   []string{"string"},
									Format: "date-time",
								}}}}},
					},
				},
				},
				"metadata": {SchemaProps: spec.SchemaProps{
					Type: []string{"object"},
					Properties: map[string]spec.Schema{
						"name": *spec.StringProperty(),
						"value": {
							SchemaProps: spec.SchemaProps{
								Type: []string{"array"},
								Items: &spec.SchemaOrArray{Schema: &spec.Schema{
									SchemaProps: spec.SchemaProps{
										Type: []string{"string"},
									}}},
							},
						},
					},
				}},
			}}}
}

type TestClient struct{}

func (_ TestClient) ResolveSchema(gvk schema.GroupVersionKind) (*spec.Schema, error) {
	return testSchema(), nil
}

func TestOpenAPITypeResolver(t *testing.T) {
	typeName := "self"

	s := schema.FromAPIVersionAndKind("v1", "CustomObject")

	resolver := resource.NewOpenAPITypeResolver(TestClient{})

	provider, err := resolver.GetDeclProvier(s, typeName)
	if err != nil {
		t.Fatal(err.Error())
	}

	env, err := compiler.NewBaseEnv()
	opts, err := provider.EnvOptions(env.CELTypeProvider())

	rootType, ok := provider.FindDeclType(typeName)
	if !ok {
		t.Fatal("declaration type not found")
	}

	opts = append(opts, cel.Variable("object", rootType.CelType()))
	env, err = env.Extend(opts...)

	ast, issue := env.Compile(`object.name != ""`)
	if issue != nil {
		t.Fatal(issue.Err().Error())
	}

	prog, err := env.Program(ast)
	if err != nil {
		t.Fatal(err.Error())
	}

	_, _, err = prog.Eval(map[string]any{
		"object": map[string]any{
			"name": "test",
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}
}

type TestClientError struct{}

func (_ TestClientError) ResolveSchema(gvk schema.GroupVersionKind) (*spec.Schema, error) {
	return nil, errors.New("dummy")
}

func TestOpenAPITypeResolverError(t *testing.T) {
	typeName := "self"
	s := schema.FromAPIVersionAndKind("v1", "CustomObject")
	resolver := resource.NewOpenAPITypeResolver(TestClientError{})
	_, err := resolver.GetDeclProvier(s, typeName)
	assert.Error(t, err)
}
