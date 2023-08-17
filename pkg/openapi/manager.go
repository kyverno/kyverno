package openapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/go-logr/logr"
	"github.com/google/gnostic/compiler"
	openapiv2 "github.com/google/gnostic/openapiv2"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	openapicontroller "github.com/kyverno/kyverno/pkg/controllers/openapi"
	"github.com/kyverno/kyverno/pkg/engine"
	cmap "github.com/orcaman/concurrent-map/v2"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"
)

type ValidateInterface interface {
	ValidateResource(unstructured.Unstructured, string, string) error
	ValidatePolicyMutation(kyvernov1.PolicyInterface) error
}

type Manager interface {
	ValidateInterface
	openapicontroller.Manager
}

type manager struct {
	// definitions holds the map of {definitionName: *openapiv2.Schema}
	definitions cmap.ConcurrentMap[string, *openapiv2.Schema]

	// kindToDefinitionName holds the map of {(group/version/)kind: definitionName}
	// i.e. with k8s 1.20.2
	// - Ingress: io.k8s.api.networking.v1.Ingress (preferred version)
	// - networking.k8s.io/v1/Ingress: io.k8s.api.networking.v1.Ingress
	// - networking.k8s.io/v1beta1/Ingress: io.k8s.api.networking.v1beta1.Ingress
	// - extension/v1beta1/Ingress: io.k8s.api.extensions.v1beta1.Ingress
	gvkToDefinitionName cmap.ConcurrentMap[string, string]

	crdList []string
	models  proto.Models

	// kindToAPIVersions stores the Kind and all its available apiVersions, {kind: apiVersions}
	kindToAPIVersions cmap.ConcurrentMap[string, apiVersions]

	logger logr.Logger
	lock   sync.Mutex
}

// apiVersions stores all available gvks for a kind, a gvk is "/" separated string
type apiVersions struct {
	serverPreferredGVK string
	gvks               []string
}

// NewManager initializes a new instance of openapi schema manager
func NewManager(logger logr.Logger) (*manager, error) {
	mgr := &manager{
		definitions:         cmap.New[*openapiv2.Schema](),
		gvkToDefinitionName: cmap.New[string](),
		kindToAPIVersions:   cmap.New[apiVersions](),
		logger:              logger,
	}

	apiResourceLists, preferredAPIResourcesLists, err := getAPIResourceLists()
	if err != nil {
		return nil, err
	}

	mgr.UpdateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists)

	defaultDoc, err := getSchemaDocument()
	if err != nil {
		return nil, err
	}

	err = mgr.UseOpenAPIDocument(defaultDoc)
	if err != nil {
		return nil, err
	}

	return mgr, nil
}

func (o *manager) Lock() {
	o.lock.Lock()
}

func (o *manager) Unlock() {
	o.lock.Unlock()
}

// ValidateResource ...
func (o *manager) ValidateResource(patchedResource unstructured.Unstructured, apiVersion, kind string) error {
	o.Lock()
	defer o.Unlock()
	return o.validateResource(patchedResource, apiVersion, kind)
}

// ValidatePolicyMutation ...
func (o *manager) ValidatePolicyMutation(policy kyvernov1.PolicyInterface) error {
	o.Lock()
	defer o.Unlock()
	kindToRules := make(map[string][]kyvernov1.Rule)
	for _, rule := range autogen.ComputeRules(policy) {
		if rule.HasMutate() {
			if rule.IsMutateExisting() {
				for _, target := range rule.Mutation.Targets {
					kindToRules[target.Kind] = append(kindToRules[target.Kind], rule)
				}
			} else {
				for _, kind := range rule.MatchResources.Kinds {
					kindToRules[kind] = append(kindToRules[kind], rule)
				}
				for _, resourceFilter := range rule.MatchResources.Any {
					for _, kind := range resourceFilter.Kinds {
						kindToRules[kind] = append(kindToRules[kind], rule)
					}
				}
				for _, resourceFilter := range rule.MatchResources.All {
					for _, kind := range resourceFilter.Kinds {
						kindToRules[kind] = append(kindToRules[kind], rule)
					}
				}
			}
		}
	}

	for kind, rules := range kindToRules {
		if kind == "CustomResourceDefinition" {
			continue
		}
		newPolicy := policy.CreateDeepCopy()
		spec := newPolicy.GetSpec()
		spec.SetRules(rules)
		k, ok := o.gvkToDefinitionName.Get(kind)
		if !ok {
			continue
		}
		d, ok := o.definitions.Get(k)
		if !ok {
			continue
		}
		resource, _ := o.generateEmptyResource(d).(map[string]interface{})
		if len(resource) == 0 {
			o.logger.V(2).Info("unable to validate resource. OpenApi definition not found", "kind", kind)
			return nil
		}

		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)

		patchedResource, err := engine.ForceMutate(nil, o.logger, newPolicy, newResource)
		if err != nil {
			return err
		}

		if kind != "*" {
			err = o.validateResource(*patchedResource.DeepCopy(), "", kind)
			if err != nil {
				return fmt.Errorf("mutate result violates resource schema: %w", err)
			}
		}
	}

	return nil
}

// ValidateResource ...
func (o *manager) validateResource(patchedResource unstructured.Unstructured, apiVersion, kind string) error {
	var err error

	gvk := kind
	if apiVersion != "" {
		gvk = apiVersion + "/" + kind
	}

	kind, _ = o.gvkToDefinitionName.Get(gvk)
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

func (o *manager) UseOpenAPIDocument(doc *openapiv2.Document) error {
	for _, definition := range doc.GetDefinitions().AdditionalProperties {
		definitionName := definition.GetName()

		o.definitions.Set(definitionName, definition.GetValue())

		gvk, preferredGVK, err := o.getGVKByDefinitionName(definitionName)
		if err != nil {
			o.logger.V(5).Info("unable to cache OpenAPISchema", "definitionName", definitionName, "reason", err.Error())
			continue
		}

		if preferredGVK {
			paths := strings.Split(definitionName, ".")
			kind := paths[len(paths)-1]
			o.gvkToDefinitionName.Set(kind, definitionName)
		}

		if gvk != "" {
			o.gvkToDefinitionName.Set(gvk, definitionName)
		}
	}

	var err error
	o.models, err = proto.NewOpenAPIData(doc)
	if err != nil {
		return err
	}

	return nil
}

func (o *manager) getGVKByDefinitionName(definitionName string) (gvk string, preferredGVK bool, err error) {
	paths := strings.Split(definitionName, ".")
	kind := paths[len(paths)-1]
	versions, ok := o.kindToAPIVersions.Get(kind)
	if !ok {
		// the kind here is the sub-resource of a K8s Kind, i.e. CronJobStatus
		// such cases are skipped in schema validation
		return
	}

	if matchGVK(definitionName, versions.serverPreferredGVK) {
		preferredGVK = true
	}

	for _, gvk := range versions.gvks {
		if matchGVK(definitionName, gvk) {
			return gvk, preferredGVK, nil
		}
	}

	return "", preferredGVK, fmt.Errorf("gvk not found by the given definition name %s, %v", definitionName, versions.gvks)
}

func (c *manager) GetCrdList() []string {
	return c.crdList
}

// UpdateKindToAPIVersions sets kindToAPIVersions with static manifests
func (c *manager) UpdateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists []*metav1.APIResourceList) {
	tempKindToAPIVersions := getAllAPIVersions(apiResourceLists)
	tempKindToAPIVersions = setPreferredVersions(tempKindToAPIVersions, preferredAPIResourcesLists)

	c.kindToAPIVersions = cmap.New[apiVersions]()
	for key, value := range tempKindToAPIVersions {
		c.kindToAPIVersions.Set(key, value)
	}
}

// For crd, we do not store definition in document
func (o *manager) getCRDSchema(kind string) (proto.Schema, error) {
	if kind == "" {
		return nil, fmt.Errorf("invalid kind")
	}

	path := proto.NewPath(kind)
	definition, _ := o.definitions.Get(kind)
	if definition == nil {
		return nil, fmt.Errorf("could not find definition")
	}

	// This was added so crd's can access
	// normal definitions from existing schema such as
	// `metadata` - this maybe a breaking change.
	// Removing this may cause policy validate to stop working
	existingDefinitions, _ := o.models.(*proto.Definitions)

	return (existingDefinitions).ParseSchema(definition, &path)
}

func (o *manager) generateEmptyResource(kindSchema *openapiv2.Schema) interface{} {
	types := kindSchema.GetType().GetValue()

	if kindSchema.GetXRef() != "" {
		d, _ := o.definitions.Get(strings.TrimPrefix(kindSchema.GetXRef(), "#/definitions/"))
		return o.generateEmptyResource(d)
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

	o.logger.V(2).Info("unknown type", types[0])
	return nil
}

func (o *manager) DeleteCRDFromPreviousSync() {
	for _, crd := range o.crdList {
		o.gvkToDefinitionName.Remove(crd)
		o.definitions.Remove(crd)
	}

	o.crdList = make([]string, 0)
}

// ParseCRD loads CRD to the cache
func (o *manager) ParseCRD(crd unstructured.Unstructured) {
	var err error

	crdRaw, _ := json.Marshal(crd.Object)
	_ = json.Unmarshal(crdRaw, &crdDefinitionPrior)

	openV3schema := crdDefinitionPrior.Spec.Validation.OpenAPIV3Schema
	crdName := crdDefinitionPrior.Spec.Names.Kind

	if openV3schema == nil {
		_ = json.Unmarshal(crdRaw, &crdDefinitionNew)
		for _, crdVersion := range crdDefinitionNew.Spec.Versions {
			if crdVersion.Storage {
				openV3schema = crdVersion.Schema.OpenAPIV3Schema
				crdName = crdDefinitionNew.Spec.Names.Kind
				break
			}
		}
	}

	if openV3schema == nil {
		o.logger.V(4).Info("skip adding schema, CRD has no properties", "name", crdName)
		return
	}

	schemaRaw, _ := json.Marshal(openV3schema)
	if len(schemaRaw) < 1 {
		o.logger.V(4).Info("failed to parse crd schema", "name", crdName)
		return
	}

	schemaRaw, err = addingDefaultFieldsToSchema(crdName, schemaRaw)
	if err != nil {
		o.logger.Error(err, "failed to parse crd schema", "name", crdName)
		return
	}

	var schema yaml.Node
	_ = yaml.Unmarshal(schemaRaw, &schema)

	parsedSchema, err := openapiv2.NewSchema(&schema, compiler.NewContext("schema", &schema, nil))
	if err != nil {
		v3valueFound := isOpenV3Error(err)
		if !v3valueFound {
			o.logger.Error(err, "failed to parse crd schema", "name", crdName)
		}
		return
	}

	o.crdList = append(o.crdList, crdName)
	o.gvkToDefinitionName.Set(crdName, crdName)
	o.definitions.Set(crdName, parsedSchema)
}
