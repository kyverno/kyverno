package openapi

import (
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"gopkg.in/yaml.v2"

	"github.com/googleapis/gnostic/compiler"

	openapi_v2 "github.com/googleapis/gnostic/OpenAPIv2"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/util/wait"
)

type crdSync struct {
	client     *client.Client
	controller *Controller
}

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
	newDoc, err := c.client.DiscoveryClient.OpenAPISchema()
	if err != nil {
		log.Log.Error(err, "cannot get openapi schema")
	}

	err = c.controller.useOpenApiDocument(newDoc)
	if err != nil {
		log.Log.Error(err, "Could not set custom OpenApi document")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.sync, time.Second*10, stopCh)
	}
	<-stopCh
}

func (c *crdSync) sync() {
	c.controller.mutex.Lock()
	defer c.controller.mutex.Unlock()

	crds, err := c.client.ListResource("CustomResourceDefinition", "", nil)
	if err != nil {
		log.Log.Error(err, "could not fetch crd's from server")
		return
	}

	c.controller.deleteCRDFromPreviousSync()

	for _, crd := range crds.Items {
		c.controller.parseCRD(crd)
	}
}

func (o *Controller) deleteCRDFromPreviousSync() {
	for _, crd := range o.crdList {
		delete(o.kindToDefinitionName, crd)
		delete(o.definitions, crd)
	}

	o.crdList = []string{}
}

func (o *Controller) parseCRD(crd unstructured.Unstructured) {
	var crdDefinition struct {
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

	crdRaw, _ := json.Marshal(crd.Object)
	_ = json.Unmarshal(crdRaw, &crdDefinition)

	crdName := crdDefinition.Spec.Names.Kind
	if len(crdDefinition.Spec.Versions) < 1 {
		log.Log.V(4).Info("could not parse crd schema, no versions present")
		return
	}

	var schema yaml.MapSlice
	schemaRaw, _ := json.Marshal(crdDefinition.Spec.Versions[0].Schema.OpenAPIV3Schema)
	_ = yaml.Unmarshal(schemaRaw, &schema)

	parsedSchema, err := openapi_v2.NewSchema(schema, compiler.NewContext("schema", nil))
	if err != nil {
		log.Log.Error(err, "could not parse crd schema:")
		return
	}

	o.crdList = append(o.crdList, crdName)

	o.kindToDefinitionName[crdName] = crdName
	o.definitions[crdName] = parsedSchema
}
