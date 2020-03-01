package openapi

import (
	"log"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/util/wait"
)

type crdSync struct {
	client *client.Client
}

func NewCRDSync(client *client.Client) *crdSync {
	return &crdSync{
		client: client,
	}
}

func (c *crdSync) Run(workers int, stopCh <-chan struct{}) {
	for i := 0; i < workers; i++ {
		go wait.Until(c.syncCrd, time.Second*10, stopCh)
	}
}

func (c *crdSync) syncCrd() {
	newDoc, _ := c.client.DiscoveryClient.OpenAPISchema()
	UseCustomOpenApiDocument(newDoc)
	x := openApiGlobalState
	log.Println(x)
}
