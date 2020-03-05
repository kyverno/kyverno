package openapi

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/nirmata/kyverno/data"

	"github.com/golang/glog"

	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/googleapis/gnostic/compiler"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"

	"gopkg.in/yaml.v2"
)

var openApiGlobalState struct {
	mutex                sync.RWMutex
	document             *openapi_v2.Document
	definitions          map[string]*openapi_v2.Schema
	kindToDefinitionName map[string]string
	crdList              []string
	models               proto.Models
}

func init() {
	defaultDoc, err := getSchemaDocument()
	if err != nil {
		panic(err)
	}

	err = useOpenApiDocument(defaultDoc)
	if err != nil {
		panic(err)
	}
}

func ValidatePolicyMutation(policy v1.ClusterPolicy) error {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()

	var kindToRules = make(map[string][]v1.Rule)
	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
			rule.MatchResources = v1.MatchResources{
				UserInfo: v1.UserInfo{},
				ResourceDescription: v1.ResourceDescription{
					Kinds: rule.MatchResources.Kinds,
				},
			}
			rule.ExcludeResources = v1.ExcludeResources{}
			for _, kind := range rule.MatchResources.Kinds {
				kindToRules[kind] = append(kindToRules[kind], rule)
			}
		}
	}

	for kind, rules := range kindToRules {
		newPolicy := *policy.DeepCopy()
		newPolicy.Spec.Rules = rules
		resource, _ := generateEmptyResource(openApiGlobalState.definitions[openApiGlobalState.kindToDefinitionName[kind]]).(map[string]interface{})
		if resource == nil {
			glog.V(4).Infof("Cannot Validate policy: openApi definition now found for %v", kind)
			return nil
		}
		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)

		ctx := context.NewContext()
		err := ctx.AddSA("kyvernoDummyUsername")
		if err != nil {
			glog.V(4).Infof("Failed to load service account in context:%v", err)
		}

		patchedResource, err := engine.ForceMutate(ctx, *newPolicy.DeepCopy(), newResource)
		if err != nil {
			return err
		}
		err = ValidateResource(*patchedResource.DeepCopy(), kind)
		if err != nil {
			return err
		}
	}

	return nil
}

func ValidateResource(patchedResource unstructured.Unstructured, kind string) error {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()
	var err error

	kind = openApiGlobalState.kindToDefinitionName[kind]
	schema := openApiGlobalState.models.LookupModel(kind)
	if schema == nil {
		schema, err = getSchemaFromDefinitions(kind)
		if err != nil || schema == nil {
			return fmt.Errorf("pre-validation: couldn't find model %s", kind)
		}
		delete(patchedResource.Object, "kind")
	}

	if errs := validation.ValidateModel(patchedResource.UnstructuredContent(), schema, kind); len(errs) > 0 {
		var errorMessages []string
		for i := range errs {
			errorMessages = append(errorMessages, errs[i].Error())
		}

		return fmt.Errorf(strings.Join(errorMessages, "\n\n"))
	}

	return nil
}

func useOpenApiDocument(customDoc *openapi_v2.Document) error {
	openApiGlobalState.mutex.Lock()
	defer openApiGlobalState.mutex.Unlock()

	openApiGlobalState.document = customDoc

	openApiGlobalState.definitions = make(map[string]*openapi_v2.Schema)
	openApiGlobalState.kindToDefinitionName = make(map[string]string)
	for _, definition := range openApiGlobalState.document.GetDefinitions().AdditionalProperties {
		openApiGlobalState.definitions[definition.GetName()] = definition.GetValue()
		path := strings.Split(definition.GetName(), ".")
		openApiGlobalState.kindToDefinitionName[path[len(path)-1]] = definition.GetName()
	}

	var err error
	openApiGlobalState.models, err = proto.NewOpenAPIData(openApiGlobalState.document)
	if err != nil {
		return err
	}

	return nil
}

func getSchemaDocument() (*openapi_v2.Document, error) {
	var spec yaml.MapSlice
	err := yaml.Unmarshal([]byte(data.SwaggerDoc), &spec)
	if err != nil {
		return nil, err
	}

	return openapi_v2.NewDocument(spec, compiler.NewContext("$root", nil))
}

// For crd, we do not store definition in document
func getSchemaFromDefinitions(kind string) (proto.Schema, error) {
	path := proto.NewPath(kind)
	return (&proto.Definitions{}).ParseSchema(openApiGlobalState.definitions[kind], &path)
}

func generateEmptyResource(kindSchema *openapi_v2.Schema) interface{} {

	types := kindSchema.GetType().GetValue()

	if kindSchema.GetXRef() != "" {
		return generateEmptyResource(openApiGlobalState.definitions[strings.TrimPrefix(kindSchema.GetXRef(), "#/definitions/")])
	}

	if len(types) != 1 {
		if len(kindSchema.GetProperties().GetAdditionalProperties()) > 0 {
			types = []string{"object"}
		} else {
			return nil
		}
	}

	switch types[0] {
	case "object":
		var props = make(map[string]interface{})
		properties := kindSchema.GetProperties().GetAdditionalProperties()
		if len(properties) == 0 {
			return props
		}

		var wg sync.WaitGroup
		var mutex sync.Mutex
		wg.Add(len(properties))
		for _, property := range properties {
			go func(property *openapi_v2.NamedSchema) {
				prop := generateEmptyResource(property.GetValue())
				mutex.Lock()
				props[property.GetName()] = prop
				mutex.Unlock()
				wg.Done()
			}(property)
		}
		wg.Wait()
		return props
	case "array":
		var array []interface{}
		for _, schema := range kindSchema.GetItems().GetSchema() {
			array = append(array, generateEmptyResource(schema))
		}
		return array
	case "string":
		if kindSchema.GetDefault() != nil {
			return string(kindSchema.GetDefault().Value.Value)
		}
		if kindSchema.GetExample() != nil {
			return string(kindSchema.GetExample().GetValue().Value)
		}
		return ""
	case "integer":
		if kindSchema.GetDefault() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetDefault().Value.Value))
			return int64(val)
		}
		if kindSchema.GetExample() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetExample().GetValue().Value))
			return int64(val)
		}
		return int64(0)
	case "number":
		if kindSchema.GetDefault() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetDefault().Value.Value))
			return int64(val)
		}
		if kindSchema.GetExample() != nil {
			val, _ := strconv.Atoi(string(kindSchema.GetExample().GetValue().Value))
			return int64(val)
		}
		return int64(0)
	case "boolean":
		if kindSchema.GetDefault() != nil {
			return string(kindSchema.GetDefault().Value.Value) == "true"
		}
		if kindSchema.GetExample() != nil {
			return string(kindSchema.GetExample().GetValue().Value) == "true"
		}
		return false
	}

	return nil
}
