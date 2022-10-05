package openapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/googleapis/gnostic/compiler"
	openapiv2 "github.com/googleapis/gnostic/openapiv2"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/data"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils"
	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kube-openapi/pkg/util/proto/validation"
)

// type concurrentMap struct{ cmap.ConcurrentMap }

type ValidateInterface interface {
	ValidateResource(resource unstructured.Unstructured, apiVersion, kind string) error
}

// Controller represents OpenAPIController
type Controller struct {
	// definitions holds the map of {definitionName: *openapiv2.Schema}
	definitions cmap.ConcurrentMap[*openapiv2.Schema]

	// kindToDefinitionName holds the map of {(group/version/)kind: definitionName}
	// i.e. with k8s 1.20.2
	// - Ingress: io.k8s.api.networking.v1.Ingress (preferred version)
	// - networking.k8s.io/v1/Ingress: io.k8s.api.networking.v1.Ingress
	// - networking.k8s.io/v1beta1/Ingress: io.k8s.api.networking.v1beta1.Ingress
	// - extension/v1beta1/Ingress: io.k8s.api.extensions.v1beta1.Ingress
	gvkToDefinitionName cmap.ConcurrentMap[string]

	crdList []string
	models  proto.Models

	// kindToAPIVersions stores the Kind and all its available apiVersions, {kind: apiVersions}
	kindToAPIVersions cmap.ConcurrentMap[apiVersions]
}

// apiVersions stores all available gvks for a kind, a gvk is "/" separated string
type apiVersions struct {
	serverPreferredGVK string
	gvks               []string
}

// NewOpenAPIController initializes a new instance of OpenAPIController
func NewOpenAPIController() (*Controller, error) {
	controller := &Controller{
		definitions:         cmap.New[*openapiv2.Schema](),
		gvkToDefinitionName: cmap.New[string](),
		kindToAPIVersions:   cmap.New[apiVersions](),
	}

	apiResourceLists, preferredAPIResourcesLists, err := getAPIResourceLists()
	if err != nil {
		return nil, err
	}

	controller.updateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists)

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

// ValidateResource ...
func (o *Controller) ValidateResource(patchedResource unstructured.Unstructured, apiVersion, kind string) error {
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

// ValidatePolicyMutation ...
func (o *Controller) ValidatePolicyMutation(policy kyvernov1.PolicyInterface) error {
	kindToRules := make(map[string][]kyvernov1.Rule)
	for _, rule := range autogen.ComputeRules(policy) {
		if rule.HasMutate() {
			for _, kind := range rule.MatchResources.Kinds {
				kindToRules[kind] = append(kindToRules[kind], rule)
			}
		}
	}

	for kind, rules := range kindToRules {
		newPolicy := policy.CreateDeepCopy()
		spec := newPolicy.GetSpec()
		spec.SetRules(rules)
		k, _ := o.gvkToDefinitionName.Get(kind)
		d, _ := o.definitions.Get(k)
		resource, _ := o.generateEmptyResource(d).(map[string]interface{})
		if len(resource) == 0 {
			logging.V(2).Info("unable to validate resource. OpenApi definition not found", "kind", kind)
			return nil
		}

		newResource := unstructured.Unstructured{Object: resource}
		newResource.SetKind(kind)

		patchedResource, err := engine.ForceMutate(nil, newPolicy, newResource)
		if err != nil {
			return err
		}

		if kind != "*" {
			err = o.ValidateResource(*patchedResource.DeepCopy(), "", kind)
			if err != nil {
				return errors.Wrapf(err, "mutate result violates resource schema")
			}
		}
	}

	return nil
}

func (o *Controller) useOpenAPIDocument(doc *openapiv2.Document) error {
	for _, definition := range doc.GetDefinitions().AdditionalProperties {
		definitionName := definition.GetName()

		o.definitions.Set(definitionName, definition.GetValue())

		gvk, preferredGVK, err := o.getGVKByDefinitionName(definitionName)
		if err != nil {
			logging.V(5).Info("unable to cache OpenAPISchema", "definitionName", definitionName, "reason", err.Error())
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

func (o *Controller) getGVKByDefinitionName(definitionName string) (gvk string, preferredGVK bool, err error) {
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

func parseGVK(str string) (group, apiVersion, kind string) {
	if strings.Count(str, "/") == 0 {
		return "", "", str
	}
	splitString := strings.Split(str, "/")
	if strings.Count(str, "/") == 1 {
		return "", splitString[0], splitString[1]
	}
	return splitString[0], splitString[1], splitString[2]
}

func groupMatches(gvkMap map[string]bool, group, kind string) bool {
	if group == "" {
		ok := gvkMap["core"]
		if ok {
			return true
		}
	} else {
		elements := strings.Split(group, ".")
		ok := gvkMap[elements[0]]
		if ok {
			return true
		}
	}
	return false
}

// matchGVK is a helper function that checks if the
// given GVK matches the definition name

func matchGVK(definitionName, gvk string) bool {
	paths := strings.Split(definitionName, ".")

	gvkMap := make(map[string]bool)
	for _, p := range paths {
		gvkMap[p] = true
	}

	group, version, kind := parseGVK(gvk)

	ok := gvkMap[kind]
	if !ok {
		return false
	}
	ok = gvkMap[version]
	if !ok {
		return false
	}

	if !groupMatches(gvkMap, group, kind) {
		return false
	}

	return true
}

// updateKindToAPIVersions sets kindToAPIVersions with static manifests
func (c *Controller) updateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists []*metav1.APIResourceList) {
	tempKindToAPIVersions := getAllAPIVersions(apiResourceLists)
	tempKindToAPIVersions = setPreferredVersions(tempKindToAPIVersions, preferredAPIResourcesLists)

	c.kindToAPIVersions = cmap.New[apiVersions]()
	for key, value := range tempKindToAPIVersions {
		c.kindToAPIVersions.Set(key, value)
	}
}

func getSchemaDocument() (*openapiv2.Document, error) {
	var spec yaml.Node
	err := yaml.Unmarshal([]byte(data.SwaggerDoc), &spec)
	if err != nil {
		return nil, err
	}

	root := spec.Content[0]
	return openapiv2.NewDocument(root, compiler.NewContext("$root", root, nil))
}

// For crd, we do not store definition in document
func (o *Controller) getCRDSchema(kind string) (proto.Schema, error) {
	if kind == "" {
		return nil, errors.New("invalid kind")
	}

	path := proto.NewPath(kind)
	definition, _ := o.definitions.Get(kind)
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

func (o *Controller) generateEmptyResource(kindSchema *openapiv2.Schema) interface{} {
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

	logging.V(2).Info("unknown type", types[0])
	return nil
}

func getArrayValue(kindSchema *openapiv2.Schema, o *Controller) interface{} {
	var array []interface{}
	for _, schema := range kindSchema.GetItems().GetSchema() {
		array = append(array, o.generateEmptyResource(schema))
	}

	return array
}

func getObjectValue(kindSchema *openapiv2.Schema, o *Controller) interface{} {
	props := make(map[string]interface{})
	properties := kindSchema.GetProperties().GetAdditionalProperties()
	if len(properties) == 0 {
		return props
	}

	var wg sync.WaitGroup
	var mutex sync.Mutex
	wg.Add(len(properties))
	for _, property := range properties {
		go func(property *openapiv2.NamedSchema) {
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

func getBoolValue(kindSchema *openapiv2.Schema) bool {
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

func getNumericValue(kindSchema *openapiv2.Schema) int64 {
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

func getStringValue(kindSchema *openapiv2.Schema) string {
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

func getAnyValue(any *openapiv2.Any) []byte {
	if any != nil {
		if val := any.GetValue(); val != nil {
			return val.GetValue()
		}
	}

	return nil
}

// getAllAPIVersions gets all available versions for a kind
// returns a map which stores all kinds with its versions
func getAllAPIVersions(apiResourceLists []*metav1.APIResourceList) map[string]apiVersions {
	tempKindToAPIVersions := make(map[string]apiVersions)

	for _, apiResourceList := range apiResourceLists {
		lastKind := ""
		for _, apiResource := range apiResourceList.APIResources {
			if apiResource.Kind == lastKind {
				continue
			}

			version, ok := tempKindToAPIVersions[apiResource.Kind]
			if !ok {
				tempKindToAPIVersions[apiResource.Kind] = apiVersions{}
			}

			gvk := strings.Join([]string{apiResourceList.GroupVersion, apiResource.Kind}, "/")
			version.gvks = append(version.gvks, gvk)
			tempKindToAPIVersions[apiResource.Kind] = version
			lastKind = apiResource.Kind
		}
	}

	return tempKindToAPIVersions
}

// setPreferredVersions sets the serverPreferredGVK of the given apiVersions map
func setPreferredVersions(kindToAPIVersions map[string]apiVersions, preferredAPIResourcesLists []*metav1.APIResourceList) map[string]apiVersions {
	tempKindToAPIVersionsCopied := copyKindToAPIVersions(kindToAPIVersions)

	for kind, versions := range tempKindToAPIVersionsCopied {
		for _, preferredAPIResourcesList := range preferredAPIResourcesLists {
			for _, resource := range preferredAPIResourcesList.APIResources {
				preferredGV := preferredAPIResourcesList.GroupVersion
				preferredGVK := preferredGV + "/" + resource.Kind

				if utils.ContainsString(versions.gvks, preferredGVK) {
					v := kindToAPIVersions[kind]

					// if a Kind belongs to multiple groups, the first group/version
					// returned from discovery docs is used as preferred version
					// https://github.com/kubernetes/kubernetes/issues/94761#issuecomment-691982480
					if v.serverPreferredGVK != "" {
						continue
					}

					v.serverPreferredGVK = strings.Join([]string{preferredGV, kind}, "/")
					kindToAPIVersions[kind] = v
				}
			}
		}
	}

	return kindToAPIVersions
}

func copyKindToAPIVersions(old map[string]apiVersions) map[string]apiVersions {
	new := make(map[string]apiVersions, len(old))
	for key, value := range old {
		new[key] = value
	}
	return new
}

func getAPIResourceLists() ([]*metav1.APIResourceList, []*metav1.APIResourceList, error) {
	var apiResourceLists []*metav1.APIResourceList
	err := json.Unmarshal([]byte(data.APIResourceLists), &apiResourceLists)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load apiResourceLists: %v", err)
	}

	var preferredAPIResourcesLists []*metav1.APIResourceList
	err = json.Unmarshal([]byte(data.APIResourceLists), &preferredAPIResourcesLists)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load preferredAPIResourcesLists: %v", err)
	}

	return apiResourceLists, preferredAPIResourcesLists, nil
}
