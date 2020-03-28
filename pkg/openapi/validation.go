package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	data "github.com/nirmata/kyverno/api"
	"github.com/nirmata/kyverno/pkg/engine/utils"

	"github.com/nirmata/kyverno/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/googleapis/gnostic/compiler"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"
	log "sigs.k8s.io/controller-runtime/pkg/log"

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

func ValidatePolicyFields(policyRaw []byte) error {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()

	var policy v1.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	if err != nil {
		return err
	}

	policyUnst, err := utils.ConvertToUnstructured(policyRaw)
	if err != nil {
		return err
	}

	err = ValidateResource(*policyUnst.DeepCopy(), "ClusterPolicy")
	if err != nil {
		return err
	}

	return validatePolicyMutation(policy)
}

func ValidateResource(patchedResource unstructured.Unstructured, kind string) error {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()
	var err error

	kind = openApiGlobalState.kindToDefinitionName[kind]
	schema := openApiGlobalState.models.LookupModel(kind)
	if schema == nil {
		// Check if kind is a CRD
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

func GetDefinitionNameFromKind(kind string) string {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()
	return openApiGlobalState.kindToDefinitionName[kind]
}

func validatePolicyMutation(policy v1.ClusterPolicy) error {
	var kindToRules = make(map[string][]v1.Rule)
	for _, rule := range policy.Spec.Rules {
		if rule.HasMutate() {
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
			log.Log.V(4).Info(fmt.Sprintf("Cannot Validate policy: openApi definition now found for %v", kind))
			return nil
		}
		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)

		patchedResource, err := engine.ForceMutate(nil, *newPolicy.DeepCopy(), newResource)
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
	if kind == "" {
		return nil, errors.New("invalid kind")
	}

	path := proto.NewPath(kind)
	definition := openApiGlobalState.definitions[kind]
	if definition == nil {
		return nil, errors.New("could not find definition")
	}

	// This was added so crd's can access
	// normal definitions from existing schema such as
	// `metadata` - this maybe a breaking change.
	// Removing this may cause policy validate to stop working
	existingDefinitions, _ := openApiGlobalState.models.(*proto.Definitions)

	return (existingDefinitions).ParseSchema(definition, &path)
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
