package data

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	"k8s.io/client-go/restmapper"
)

const crdsFolder = "crds"

//go:embed crds
var crdsFs embed.FS

//go:embed api-group-resources.json
var apiGroupResources []byte

// // APIGroupResource for --crd flag
// var apiGroupResource *restmapper.APIGroupResources

var _apiGroupResources = sync.OnceValues(func() ([]*restmapper.APIGroupResources, error) {
	var out []*restmapper.APIGroupResources
	err := json.Unmarshal(apiGroupResources, &out)
	return out, err
})

func Crds() (fs.FS, error) {
	return fs.Sub(crdsFs, crdsFolder)
}

func APIGroupResources() ([]*restmapper.APIGroupResources, error) {
	return _apiGroupResources()
}

type CRDProcessor struct {
	apiGroupResource *restmapper.APIGroupResources
	mutex            sync.RWMutex
}

func NewCRDProcessor(resources *restmapper.APIGroupResources) *CRDProcessor {
	return &CRDProcessor{
		apiGroupResource: resources,
	}
}

func (p *CRDProcessor) AddResourceGroup(resources *restmapper.APIGroupResources) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.apiGroupResource = resources
}

func (p *CRDProcessor) GetResourceGroup() (*restmapper.APIGroupResources, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if p.apiGroupResource == nil {
		return nil, fmt.Errorf("CRD API group resources not initialized")
	}

	return p.apiGroupResource, nil
}
