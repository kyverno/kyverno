// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package surface_v1

import (
	"errors"
	"fmt"
	"log"

	"strings"

	openapiv3 "github.com/googleapis/gnostic/OpenAPIv3"
)

var knownTypes = map[string]bool{"string": true, "integer": true, "number": true, "boolean": true, "array": true, "object": true}

// NewModelFromOpenAPIv3 builds a model of an API service for use in code generation.
func NewModelFromOpenAPI3(document *openapiv3.Document) (*Model, error) {
	return newOpenAPI3Builder().buildModel(document)
}

type OpenAPI3Builder struct {
	model *Model
}

func newOpenAPI3Builder() *OpenAPI3Builder {
	return &OpenAPI3Builder{model: &Model{}}
}

func (b *OpenAPI3Builder) buildModel(document *openapiv3.Document) (*Model, error) {
	// Set model properties from passed-in document.
	b.model.Name = document.Info.Title
	b.model.Types = make([]*Type, 0)
	b.model.Methods = make([]*Method, 0)
	err := b.build(document)
	if err != nil {
		return nil, err
	}
	return b.model, nil
}

// build builds an API service description, preprocessing its types and methods for code generation.
func (b *OpenAPI3Builder) build(document *openapiv3.Document) (err error) {
	err = b.buildTypesFromComponents(document.Components)
	if err != nil {
		return err
	}

	// Collect service method descriptions from each PathItem.
	if document.Paths != nil {
		for _, pair := range document.Paths.Path {
			b.buildMethodFromPathItem(pair.Name, pair.Value)
		}
	}
	return err
}

// buildTypesFromComponents builds multiple service type description from the "Components" section
// in the OpenAPI specification.
func (b *OpenAPI3Builder) buildTypesFromComponents(components *openapiv3.Components) (err error) {
	if components == nil {
		return nil
	}

	// Collect service type descriptions from Components/Schemas.
	if components.Schemas != nil {
		for _, pair := range components.Schemas.AdditionalProperties {
			t, err := b.buildTypeFromSchemaOrReference(pair.Name, pair.Value)
			if err != nil {
				return err
			}
			if t != nil {
				b.model.addType(t)
			}
		}
	}
	// Collect service type descriptions from Components/Parameters.
	if components.Parameters != nil {
		for _, pair := range components.Parameters.AdditionalProperties {
			parameters := []*openapiv3.ParameterOrReference{pair.Value}
			_, err := b.buildTypeFromParameters(pair.Name, parameters, nil, true)
			if err != nil {
				return err
			}
		}
	}
	// Collect service type descriptions from Components/requestBodies
	if components.RequestBodies != nil {
		for _, pair := range components.RequestBodies.AdditionalProperties {
			t, err := b.buildTypeFromRequestBody(pair.Name, pair.Value, nil)

			if err != nil {
				return err
			}
			if t != nil {
				b.model.addType(t)
			}
		}
	}
	// Collect service type descriptions from Components/responses
	if components.Responses != nil {
		for _, pair := range components.Responses.AdditionalProperties {
			namedResponseOrReference := []*openapiv3.NamedResponseOrReference{pair}
			responses := &openapiv3.Responses{ResponseOrReference: namedResponseOrReference}
			b.buildTypeFromResponses(pair.Name, responses, true)
		}
	}

	return err
}

// buildTypeFromSchemaOrReference builds a service type description from a schema in the API description.
func (b *OpenAPI3Builder) buildTypeFromSchemaOrReference(
	name string,
	schemaOrReference *openapiv3.SchemaOrReference) (t *Type, err error) {
	if schema := schemaOrReference.GetSchema(); schema != nil {
		t = &Type{}
		t.Name = name
		t.Description = "implements the service definition of " + name
		t.Fields = make([]*Field, 0)
		if schema.Properties != nil {
			if len(schema.Properties.AdditionalProperties) > 0 {
				// If the schema has properties, generate a struct.
				t.Kind = TypeKind_STRUCT
			}
			for _, pair := range schema.Properties.AdditionalProperties {
				if schema := pair.Value; schema != nil {
					f := &Field{
						Name:      pair.Name,
						Serialize: true,
					}
					f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(schema)
					t.addField(f)
				}
			}
		}
		if len(t.Fields) == 0 {
			if schema.AdditionalProperties != nil {
				// If the schema has no fixed properties and additional properties of a specified type,
				// generate a map pointing to objects of that type.
				t.Kind = TypeKind_OBJECT
				t.ContentType = typeForRef(schema.AdditionalProperties.GetSchemaOrReference().GetReference().GetXRef())
			}
		}
		return t, err
	} else {
		return nil, errors.New("unable to determine service type for referenced schema " + name)
	}
}

// buildMethodFromOperation builds a service method description
func (b *OpenAPI3Builder) buildMethodFromPathItem(
	path string,
	pathItem *openapiv3.PathItem) (err error) {
	for _, method := range []string{"GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD", "PATCH", "TRACE"} {
		var op *openapiv3.Operation
		switch method {
		case "GET":
			op = pathItem.Get
		case "PUT":
			op = pathItem.Put
		case "POST":
			op = pathItem.Post
		case "DELETE":
			op = pathItem.Delete
		case "OPTIONS":
			op = pathItem.Options
		case "HEAD":
			op = pathItem.Head
		case "PATCH":
			op = pathItem.Patch
		case "TRACE":
			op = pathItem.Trace
		}
		if op != nil {
			m := &Method{
				Operation:   op.OperationId,
				Path:        path,
				Method:      method,
				Name:        sanitizeOperationName(op.OperationId),
				Description: op.Description,
			}
			if m.Name == "" {
				m.Name = generateOperationName(method, path)
			}
			m.ParametersTypeName, err = b.buildTypeFromParameters(m.Name, op.Parameters, op.RequestBody, false)
			m.ResponsesTypeName, err = b.buildTypeFromResponses(m.Name, op.Responses, false)
			b.model.addMethod(m)
		}
	}
	return err
}

// buildTypeFromParameters builds a service type description from the parameters of an API method
func (b *OpenAPI3Builder) buildTypeFromParameters(
	name string,
	parameters []*openapiv3.ParameterOrReference,
	requestBody *openapiv3.RequestBodyOrReference,
	fromComponent bool) (typeName string, err error) {

	pName := name + "Parameters"
	t := &Type{
		Name:        pName,
		Kind:        TypeKind_STRUCT,
		Fields:      make([]*Field, 0),
		Description: pName + " holds parameters to " + name,
	}

	if fromComponent {
		t.Name = name
		t.Description = t.Name + " is a parameter"
	}
	for _, parametersItem := range parameters {
		f := Field{
			Type: fmt.Sprintf("%+v", parametersItem),
		}
		parameter := parametersItem.GetParameter()
		if parameter != nil {
			switch parameter.In {
			case "body":
				f.Position = Position_BODY
			case "header":
				f.Position = Position_HEADER
			case "formdata":
				f.Position = Position_FORMDATA
			case "query":
				f.Position = Position_QUERY
			case "path":
				f.Position = Position_PATH
			}
			f.Name = parameter.Name
			if parameter.GetSchema() != nil && parameter.GetSchema() != nil {
				f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(parameter.GetSchema())
			}
			f.Serialize = true
			t.addField(&f)
		} else if parameterRef := parametersItem.GetReference(); parameterRef != nil {
			f.Type = typeForRef(parameterRef.GetXRef())
			f.Name = strings.ToLower(f.Type)
			f.Kind = FieldKind_REFERENCE
			t.addField(&f)
		}
	}

	_, err = b.buildTypeFromRequestBody(name, requestBody, t)

	if len(t.Fields) > 0 {
		b.model.addType(t)
		return t.Name, err
	}
	return "", err
}

// buildTypeFromRequestBody builds a service type description from the request bodies of an OpenAPI
// description. If tIn is not given, a new type is created. Otherwise tIn is used.
func (b *OpenAPI3Builder) buildTypeFromRequestBody(name string, requestBody *openapiv3.RequestBodyOrReference, tIn *Type) (tOut *Type, err error) {
	tOut = &Type{
		Name: name,
	}

	if tIn != nil {
		tOut = tIn
	}

	if requestBody != nil {
		content := requestBody.GetRequestBody().GetContent()
		f := &Field{
			Position:  Position_BODY,
			Serialize: true,
		}

		if content != nil {
			for _, pair := range content.GetAdditionalProperties() {
				if pair.Name != "application/json" {
					log.Printf("unimplemented: %q requestBody(%s)", name, pair.Name)
					continue
				}
				f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(pair.GetValue().GetSchema())
				f.Name = strings.ToLower(f.Type) // use the schema name as the parameter name, since none is directly specified
				tOut.addField(f)
			}
		} else if reference := requestBody.GetReference(); reference != nil {
			schemaOrReference := openapiv3.SchemaOrReference{&openapiv3.SchemaOrReference_Reference{Reference: reference}}
			f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(&schemaOrReference)
			f.Name = strings.ToLower(f.Type) // use the schema name as the parameter name, since none is directly specified
			tOut.addField(f)
		}
	}

	return tOut, err
}

// buildTypeFromResponses builds a service type description from the responses of an API method
func (b *OpenAPI3Builder) buildTypeFromResponses(
	name string,
	responses *openapiv3.Responses,
	fromComponent bool) (typeName string, err error) {

	rName := name + "Responses"
	t := &Type{
		Name:        name + "Responses",
		Kind:        TypeKind_STRUCT,
		Fields:      make([]*Field, 0),
		Description: rName + " holds responses of " + name,
	}

	if fromComponent {
		t.Name = name
		t.Description = t.Name + " is a response"
	}

	addResponse := func(name string, value *openapiv3.ResponseOrReference) {
		f := Field{
			Name:      name,
			Serialize: false,
		}
		response := value.GetResponse()
		if response != nil && response.GetContent() != nil {
			for _, pair2 := range response.GetContent().GetAdditionalProperties() {
				f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(pair2.GetValue().GetSchema())
				t.addField(&f)
			}
		} else if responseRef := value.GetReference(); responseRef != nil {
			schemaOrReference := openapiv3.SchemaOrReference{&openapiv3.SchemaOrReference_Reference{Reference: responseRef}}
			f.Kind, f.Type, f.Format = b.typeForSchemaOrReference(&schemaOrReference)
			t.addField(&f)
		}
	}

	for _, pair := range responses.ResponseOrReference {
		addResponse(pair.Name, pair.Value)
	}
	if responses.Default != nil {
		addResponse("default", responses.Default)
	}

	if len(t.Fields) > 0 {
		b.model.addType(t)
		return t.Name, err
	}
	return "", err
}

// typeForSchemaOrReference determines the language-specific type of a schema or reference
func (b *OpenAPI3Builder) typeForSchemaOrReference(value *openapiv3.SchemaOrReference) (kind FieldKind, typeName, format string) {
	if value.GetSchema() != nil {
		return b.typeForSchema(value.GetSchema())
	}
	if value.GetReference() != nil {
		return FieldKind_REFERENCE, typeForRef(value.GetReference().XRef), ""
	}
	return FieldKind_SCALAR, "todo", ""
}

// typeForSchema determines the language-specific type of a schema
func (b *OpenAPI3Builder) typeForSchema(schema *openapiv3.Schema) (kind FieldKind, typeName, format string) {
	if schema.Type != "" {
		format := schema.Format
		switch schema.Type {
		case "string":
			return FieldKind_SCALAR, "string", format
		case "integer":
			return FieldKind_SCALAR, "integer", format
		case "number":
			return FieldKind_SCALAR, "number", format
		case "boolean":
			return FieldKind_SCALAR, "boolean", format
		case "array":
			if schema.Items != nil {
				// we have an array.., but of what?
				items := schema.Items
				if items != nil {
					a := items.GetSchemaOrReference()
					if a[0].GetReference().GetXRef() != "" {
						return FieldKind_ARRAY, typeForRef(a[0].GetReference().GetXRef()), format
					} else if knownTypes[a[0].GetSchema().Type] {
						// The items of the array is one of the known types.
						return FieldKind_ARRAY, a[0].GetSchema().Type, a[0].GetSchema().Format
					}
				}
			}
		case "object":
			if schema.AdditionalProperties == nil {
				return FieldKind_MAP, "object", format
			}
		case "any":
			return FieldKind_ANY, "any", format
		default:

		}
	}
	if schema.AdditionalProperties != nil {
		additionalProperties := schema.AdditionalProperties
		if propertySchema := additionalProperties.GetSchemaOrReference().GetReference(); propertySchema != nil {
			if ref := propertySchema.XRef; ref != "" {
				return FieldKind_MAP, "map[string]" + typeForRef(ref), ""
			}
		}
	}
	// this function is incomplete... use generic interface{} for now
	log.Printf("unimplemented: %v", schema)
	return FieldKind_SCALAR, "object", ""
}
