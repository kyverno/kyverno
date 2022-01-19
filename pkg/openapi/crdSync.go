package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/googleapis/gnostic/compiler"
	openapiv2 "github.com/googleapis/gnostic/openapiv2"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type crdSync struct {
	client     *client.Client
	controller *Controller
}

// crdDefinitionPrior represents CRDs version prior to 1.16
var crdDefinitionPrior struct {
	Spec struct {
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Validation struct {
			OpenAPIV3Schema interface{} `json:"openAPIV3Schema"`
		} `json:"validation"`
	} `json:"spec"`
}

// crdDefinitionNew represents CRDs version 1.16+
var crdDefinitionNew struct {
	Spec struct {
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Schema struct {
				OpenAPIV3Schema interface{} `json:"openAPIV3Schema"`
			} `json:"schema"`
			Storage bool `json:"storage"`
		} `json:"versions"`
	} `json:"spec"`
}

// NewCRDSync ...
func NewCRDSync(client *client.Client, controller *Controller) *crdSync {
	if controller == nil {
		panic(fmt.Errorf("nil controller sent into crd sync"))
	}

	return &crdSync{
		controller: controller,
		client:     client,
	}
}

func (c *crdSync) Run(workers int, stopCh <-chan struct{}) {
	if err := c.updateInClusterKindToAPIVersions(); err != nil {
		log.Log.Error(err, "failed to update in-cluster api versions")
	}

	newDoc, err := c.client.DiscoveryClient.DiscoveryCache().OpenAPISchema()
	if err != nil {
		log.Log.Error(err, "cannot get OpenAPI schema")
	}

	err = c.controller.useOpenAPIDocument(newDoc)
	if err != nil {
		log.Log.Error(err, "Could not set custom OpenAPI document")
	}
	// Sync CRD before kyverno starts
	c.sync()

	for i := 0; i < workers; i++ {
		go wait.Until(c.sync, 15*time.Second, stopCh)
	}
}

func (c *crdSync) sync() {
	c.client.DiscoveryClient.DiscoveryCache().Invalidate()
	crds, err := c.client.GetDynamicInterface().Resource(runtimeSchema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		log.Log.Error(err, "could not fetch crd's from server")
		return
	}

	c.controller.deleteCRDFromPreviousSync()

	for _, crd := range crds.Items {
		c.controller.ParseCRD(crd)
	}

	if err := c.updateInClusterKindToAPIVersions(); err != nil {
		log.Log.Error(err, "sync failed, unable to update in-cluster api versions")
	}

	newDoc, err := c.client.DiscoveryClient.DiscoveryCache().OpenAPISchema()
	if err != nil {
		log.Log.Error(err, "cannot get OpenAPI schema")
	}

	err = c.controller.useOpenAPIDocument(newDoc)
	if err != nil {
		log.Log.Error(err, "Could not set custom OpenAPI document")
	}
}

func (c *crdSync) updateInClusterKindToAPIVersions() error {
	_, apiResourceLists, err := c.client.DiscoveryClient.DiscoveryCache().ServerGroupsAndResources()
	if err != nil {
		return errors.Wrapf(err, "fetching API server groups and resources")
	}

	preferredAPIResourcesLists, err := c.client.DiscoveryClient.DiscoveryCache().ServerPreferredResources()
	if err != nil {
		return errors.Wrapf(err, "fetching API server preferreds resources")
	}

	c.controller.updateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists)
	return nil
}

func (o *Controller) deleteCRDFromPreviousSync() {
	for _, crd := range o.crdList {
		o.gvkToDefinitionName.Remove(crd)
		o.definitions.Remove(crd)
	}

	o.crdList = make([]string, 0)
}

// ParseCRD loads CRD to the cache
func (o *Controller) ParseCRD(crd unstructured.Unstructured) {
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
		log.Log.V(4).Info("skip adding schema, CRD has no properties", "name", crdName)
		return
	}

	schemaRaw, _ := json.Marshal(openV3schema)
	if len(schemaRaw) < 1 {
		log.Log.V(4).Info("failed to parse crd schema", "name", crdName)
		return
	}

	schemaRaw, err = addingDefaultFieldsToSchema(crdName, schemaRaw)
	if err != nil {
		log.Log.Error(err, "failed to parse crd schema", "name", crdName)
		return
	}

	var schema yaml.Node
	_ = yaml.Unmarshal(schemaRaw, &schema)

	parsedSchema, err := openapiv2.NewSchema(&schema, compiler.NewContext("schema", &schema, nil))
	if err != nil {
		v3valueFound := isOpenV3Error(err)
		if v3valueFound == false {
			log.Log.Error(err, "failed to parse crd schema", "name", crdName)
		}
		return
	}

	o.crdList = append(o.crdList, crdName)
	o.gvkToDefinitionName.Set(crdName, crdName)
	o.definitions.Set(crdName, parsedSchema)
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
		log.Log.V(4).Info("crd schema has no properties", "name", crdName)
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
