package openapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/google/gnostic/compiler"
	openapiv2 "github.com/google/gnostic/openapiv2"
	"github.com/kyverno/kyverno/data"
	"github.com/kyverno/kyverno/pkg/logging"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

// matchGVK is a helper function that checks if the given GVK matches the definition name
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

func getSchemaDocument() (*openapiv2.Document, error) {
	var spec yaml.Node
	err := yaml.Unmarshal([]byte(data.SwaggerDoc), &spec)
	if err != nil {
		return nil, err
	}

	root := spec.Content[0]
	return openapiv2.NewDocument(root, compiler.NewContext("$root", root, nil))
}

func getArrayValue(kindSchema *openapiv2.Schema, o *manager) interface{} {
	var array []interface{}
	for _, schema := range kindSchema.GetItems().GetSchema() {
		array = append(array, o.generateEmptyResource(schema))
	}

	return array
}

func getObjectValue(kindSchema *openapiv2.Schema, o *manager) interface{} {
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

				if slices.Contains(versions.gvks, preferredGVK) {
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

func isOpenV3Error(err error) bool {
	unsupportedValues := []string{"anyOf", "allOf", "not"}
	v3valueFound := false
	for _, value := range unsupportedValues {
		if !strings.Contains(err.Error(), fmt.Sprintf("has invalid property: %s", value)) {
			v3valueFound = true
			break
		}
	}
	return v3valueFound
}

// addingDefaultFieldsToSchema will add any default missing fields like apiVersion, metadata
func addingDefaultFieldsToSchema(crdName string, schemaRaw []byte) ([]byte, error) {
	var schema struct {
		Properties map[string]interface{} `json:"properties"`
	}
	_ = json.Unmarshal(schemaRaw, &schema)

	if len(schema.Properties) < 1 {
		logging.V(6).Info("crd schema has no properties", "name", crdName)
		return schemaRaw, nil
	}

	if schema.Properties["apiVersion"] == nil {
		apiVersionDefRaw := `{"description":"APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources","type":"string"}`
		apiVersionDef := make(map[string]interface{})
		_ = json.Unmarshal([]byte(apiVersionDefRaw), &apiVersionDef)
		schema.Properties["apiVersion"] = apiVersionDef
	}

	if schema.Properties["metadata"] == nil {
		metadataDefRaw := `{"$ref":"#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta","description":"Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata"}`
		metadataDef := make(map[string]interface{})
		_ = json.Unmarshal([]byte(metadataDefRaw), &metadataDef)
		schema.Properties["metadata"] = metadataDef
	}

	schemaWithDefaultFields, _ := json.Marshal(schema)

	return schemaWithDefaultFields, nil
}
