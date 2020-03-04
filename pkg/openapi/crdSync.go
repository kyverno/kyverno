package openapi

import (
	"time"

	"github.com/golang/glog"

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
	<-stopCh
}

func (c *crdSync) syncCrd() {
	newDoc, err := c.client.DiscoveryClient.OpenAPISchema()
	if err != nil {
		glog.V(4).Infof("cannot get openapi schema: %v", err)
	}

	err = useCustomOpenApiDocument(newDoc)
	if err != nil {
		glog.V(4).Infof("Could not set custom OpenApi document: %v\n", err)
	}
}
