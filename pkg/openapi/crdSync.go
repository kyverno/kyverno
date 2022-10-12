package openapi

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/metrics"
	util "github.com/kyverno/kyverno/pkg/utils"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
)

type crdSync struct {
	client  dclient.Interface
	manager *Manager
}

const (
	skipErrorMsg = "Got empty response for"
)

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
func NewCRDSync(client dclient.Interface, mgr *Manager) *crdSync {
	if mgr == nil {
		panic(fmt.Errorf("nil manager sent into crd sync"))
	}

	return &crdSync{
		manager: mgr,
		client:  client,
	}
}

func (c *crdSync) Run(ctx context.Context, workers int) {
	if err := c.updateInClusterKindToAPIVersions(); err != nil {
		logging.Error(err, "failed to update in-cluster api versions")
	}

	newDoc, err := c.client.Discovery().OpenAPISchema()
	if err != nil {
		logging.Error(err, "cannot get OpenAPI schema")
	}

	err = c.manager.useOpenAPIDocument(newDoc)
	if err != nil {
		logging.Error(err, "Could not set custom OpenAPI document")
	}
	// Sync CRD before kyverno starts
	c.sync()
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.CheckSync, 15*time.Second)
	}
}

func (c *crdSync) sync() {
	c.client.Discovery().DiscoveryCache().Invalidate()
	crds, err := c.client.GetDynamicInterface().Resource(runtimeSchema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).List(context.TODO(), metav1.ListOptions{})

	c.client.RecordClientQuery(metrics.ClientList, metrics.KubeDynamicClient, "CustomResourceDefinition", "")
	if err != nil {
		logging.Error(err, "could not fetch crd's from server")
		return
	}

	c.manager.deleteCRDFromPreviousSync()

	for _, crd := range crds.Items {
		c.manager.ParseCRD(crd)
	}

	if err := c.updateInClusterKindToAPIVersions(); err != nil {
		logging.Error(err, "sync failed, unable to update in-cluster api versions")
	}

	newDoc, err := c.client.Discovery().OpenAPISchema()
	if err != nil {
		logging.Error(err, "cannot get OpenAPI schema")
	}

	err = c.manager.useOpenAPIDocument(newDoc)
	if err != nil {
		logging.Error(err, "Could not set custom OpenAPI document")
	}
}

func (c *crdSync) updateInClusterKindToAPIVersions() error {
	util.OverrideRuntimeErrorHandler()
	_, apiResourceLists, err := discovery.ServerGroupsAndResources(c.client.Discovery().DiscoveryInterface())

	if err != nil && !strings.Contains(err.Error(), skipErrorMsg) {
		return errors.Wrapf(err, "fetching API server groups and resources")
	}
	preferredAPIResourcesLists, err := discovery.ServerPreferredResources(c.client.Discovery().DiscoveryInterface())
	if err != nil && !strings.Contains(err.Error(), skipErrorMsg) {
		return errors.Wrapf(err, "fetching API server preferreds resources")
	}

	c.manager.updateKindToAPIVersions(apiResourceLists, preferredAPIResourcesLists)
	return nil
}

func (c *crdSync) CheckSync(ctx context.Context) {
	crds, err := c.client.GetDynamicInterface().Resource(runtimeSchema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}).List(ctx, metav1.ListOptions{})
	if err != nil {
		logging.Error(err, "could not fetch crd's from server")
		return
	}
	if len(c.manager.crdList) != len(crds.Items) {
		c.sync()
	}
}
