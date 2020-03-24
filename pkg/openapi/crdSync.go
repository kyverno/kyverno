package openapi

import (
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/golang/glog"

	"gopkg.in/yaml.v2"

	"github.com/googleapis/gnostic/compiler"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"

	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/util/wait"
)

type crdDefinition struct {
	Spec struct {
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Schema struct {
				OpenAPIV3Schema interface{} `json:"openAPIV3Schema"`
			} `json:"schema"`
		} `json:"versions"`
	} `json:"spec"`
}

type crdSync struct {
	client *client.Client
}

func NewCRDSync(client *client.Client) *crdSync {
	return &crdSync{
		client: client,
	}
}

func (c *crdSync) Run(workers int, stopCh <-chan struct{}) {
	newDoc, err := c.client.DiscoveryClient.OpenAPISchema()
	if err != nil {
		glog.V(4).Infof("cannot get openapi schema: %v", err)
	}

	err = useOpenApiDocument(newDoc)
	if err != nil {
		glog.V(4).Infof("Could not set custom OpenApi document: %v\n", err)
	}

	// Sync CRD before kyverno starts
	c.sync()

	for i := 0; i < workers; i++ {
		go wait.Until(c.sync, time.Second*10, stopCh)
	}
}

func (c *crdSync) sync() {
	crds, err := c.client.ListResource("CustomResourceDefinition", "", nil)
	if err != nil {
		glog.V(4).Infof("could not fetch crd's from server: %v", err)
		return
	}

	openApiGlobalState.mutex.Lock()
	defer openApiGlobalState.mutex.Unlock()

	deleteCRDFromPreviousSync()

	for _, crd := range crds.Items {
		parseCRD(crd)
	}
}

func deleteCRDFromPreviousSync() {
	for _, crd := range openApiGlobalState.crdList {
		delete(openApiGlobalState.kindToDefinitionName, crd)
		delete(openApiGlobalState.definitions, crd)
	}

	openApiGlobalState.crdList = []string{}
}

func parseCRD(crd unstructured.Unstructured) {
	var crdDefinition crdDefinition
	crdRaw, _ := json.Marshal(crd.Object)
	_ = json.Unmarshal(crdRaw, &crdDefinition)

	crdName := crdDefinition.Spec.Names.Kind
	if len(crdDefinition.Spec.Versions) < 1 {
		glog.V(4).Infof("could not parse crd schema, no versions present")
		return
	}

	var schema yaml.MapSlice
	schemaRaw, _ := json.Marshal(crdDefinition.Spec.Versions[0].Schema.OpenAPIV3Schema)
	schemaRaw = addingDefaultFieldsToSchema(schemaRaw)
	_ = yaml.Unmarshal(schemaRaw, &schema)

	parsedSchema, err := openapi_v2.NewSchema(schema, compiler.NewContext("schema", nil))
	if err != nil {
		glog.V(4).Infof("could not parse crd schema:%v", err)
		return
	}

	openApiGlobalState.crdList = append(openApiGlobalState.crdList, crdName)

	openApiGlobalState.kindToDefinitionName[crdName] = crdName
	openApiGlobalState.definitions[crdName] = parsedSchema
}

// addingDefaultFieldsToSchema will add any default missing fields like apiVersion, metadata
func addingDefaultFieldsToSchema(schemaRaw []byte) []byte {
	var schema struct {
		Properties map[string]interface{} `json:"properties"`
	}
	_ = json.Unmarshal(schemaRaw, &schema)

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

	return schemaWithDefaultFields
}
