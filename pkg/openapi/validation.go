package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	data "github.com/kyverno/kyverno/api"
	"github.com/kyverno/kyverno/pkg/engine/utils"

	"github.com/kyverno/kyverno/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	"github.com/googleapis/gnostic/compiler"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	"gopkg.in/yaml.v2"
)

// Controller represents OpenAPIController
type Controller struct {
	mutex       sync.RWMutex
	definitions map[string]*openapi_v2.Schema
	// kindToDefinitionName holds the kind - definition map
	// i.e. - Namespace: io.k8s.api.core.v1.Namespace
	kindToDefinitionName map[string]string
	crdList              []string
	models               proto.Models
}

// NewOpenAPIController initializes a new instance of OpenAPIController
func NewOpenAPIController() (*Controller, error) {
	controller := &Controller{
		definitions:          make(map[string]*openapi_v2.Schema),
		kindToDefinitionName: make(map[string]string),
	}

	defaultDoc, err := getSchemaDocument()
	if err != nil {
		return nil, err
	}

	err = controller.useOpenAPIDocument(defaultDoc)
	if err != nil {
		return nil, err
	}

	return controller, nil
}

// ValidatePolicyFields ...
func (o *Controller) ValidatePolicyFields(policyRaw []byte) error {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	var policy v1.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	if err != nil {
		return err
	}

	policyUnst, err := utils.ConvertToUnstructured(policyRaw)
	if err != nil {
		return err
	}

	err = o.ValidateResource(*policyUnst.DeepCopy(), "ClusterPolicy")
	if err != nil {
		return err
	}

	return o.ValidatePolicyMutation(policy)
}

// ValidateResource ...
func (o *Controller) ValidateResource(patchedResource unstructured.Unstructured, kind string) error {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	var err error

	kind = o.kindToDefinitionName[kind]
	schema := o.models.LookupModel(kind)
	if schema == nil {
		// Check if kind is a CRD
		schema, err = o.getCRDSchema(kind)
		if err != nil || schema == nil {
			return fmt.Errorf("pre-validation: couldn't find model %s, err: %v", kind, err)
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

// GetDefinitionNameFromKind ...
func (o *Controller) GetDefinitionNameFromKind(kind string) string {
	o.mutex.RLock()
	defer o.mutex.RUnlock()
	return o.kindToDefinitionName[kind]
}

// ValidatePolicyMutation ...
func (o *Controller) ValidatePolicyMutation(policy v1.ClusterPolicy) error {
	o.mutex.RLock()
	defer o.mutex.RUnlock()

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
		resource, _ := o.generateEmptyResource(o.definitions[o.kindToDefinitionName[kind]]).(map[string]interface{})
		if resource == nil || len(resource) == 0 {
			log.Log.V(2).Info("unable to validate resource. OpenApi definition not found", "kind", kind)
			return nil
		}

		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)

		patchedResource, err := engine.ForceMutate(nil, newPolicy, newResource)
		if err != nil {
			return err
		}

		err = o.ValidateResource(*patchedResource.DeepCopy(), kind)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *Controller) useOpenAPIDocument(doc *openapi_v2.Document) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	for _, definition := range doc.GetDefinitions().AdditionalProperties {
		o.definitions[definition.GetName()] = definition.GetValue()
		path := strings.Split(definition.GetName(), ".")
		o.kindToDefinitionName[path[len(path)-1]] = definition.GetName()
	}

	var err error
	o.models, err = proto.NewOpenAPIData(doc)
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
func (o *Controller) getCRDSchema(kind string) (proto.Schema, error) {
	if kind == "" {
		return nil, errors.New("invalid kind")
	}

	path := proto.NewPath(kind)
	definition := o.definitions[kind]
	if definition == nil {
		return nil, errors.New("could not find definition")
	}

	// This was added so crd's can access
	// normal definitions from existing schema such as
	// `metadata` - this maybe a breaking change.
	// Removing this may cause policy validate to stop working
	existingDefinitions, _ := o.models.(*proto.Definitions)

	return (existingDefinitions).ParseSchema(definition, &path)
}

func (o *Controller) generateEmptyResource(kindSchema *openapi_v2.Schema) interface{} {

	types := kindSchema.GetType().GetValue()

	if kindSchema.GetXRef() != "" {
		return o.generateEmptyResource(o.definitions[strings.TrimPrefix(kindSchema.GetXRef(), "#/definitions/")])
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
		return getObjectValue(kindSchema, o)
	case "array":
		return getArrayValue(kindSchema, o)
	case "string":
		return getStringValue(kindSchema)
	case "integer":
		return getNumericValue(kindSchema)
	case "number":
		return getNumericValue(kindSchema)
	case "boolean":
		return getBoolValue(kindSchema)
	}

	log.Log.Info("unknown type", types[0])
	return nil
}

func getArrayValue(kindSchema *openapi_v2.Schema, o *Controller) interface{} {
	var array []interface{}
	for _, schema := range kindSchema.GetItems().GetSchema() {
		array = append(array, o.generateEmptyResource(schema))
	}

	return array
}

func getObjectValue(kindSchema *openapi_v2.Schema, o *Controller) interface{} {
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
			prop := o.generateEmptyResource(property.GetValue())
			mutex.Lock()
			props[property.GetName()] = prop
			mutex.Unlock()
			wg.Done()
		}(property)
	}
	wg.Wait()
	return props
}

func getBoolValue(kindSchema *openapi_v2.Schema) bool {
	if d := kindSchema.GetDefault(); d != nil {
		v := getAnyValue(d)
		return string(v) == "true"
	}

	if e := kindSchema.GetExample(); e != nil {
		v := getAnyValue(e)
		return string(v) == "true"
	}

	return false
}

func getNumericValue(kindSchema *openapi_v2.Schema) int64 {
	if d := kindSchema.GetDefault(); d != nil {
		v := getAnyValue(d)
		val, _ := strconv.Atoi(string(v))
		return int64(val)
	}

	if e := kindSchema.GetExample(); e != nil {
		v := getAnyValue(e)
		val, _ := strconv.Atoi(string(v))
		return int64(val)
	}

	return int64(0)
}

func getStringValue(kindSchema *openapi_v2.Schema) string {
	if d := kindSchema.GetDefault(); d != nil {
		v := getAnyValue(d)
		return string(v)
	}

	if e := kindSchema.GetExample(); e != nil {
		v := getAnyValue(e)
		return string(v)
	}

	return ""
}

func getAnyValue(any *openapi_v2.Any) []byte {
	if any != nil {
		if val := any.GetValue(); val != nil {
			return val.GetValue()
		}
	}

	return nil
}
