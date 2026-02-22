package data

import (
	"embed"
	"encoding/json"
	"io/fs"
	"sync"

	"k8s.io/client-go/restmapper"
)

const crdsFolder = "crds"

//go:embed crds
var crdsFs embed.FS

//go:embed api-group-resources.json
var apiGroupResources []byte

// APIGroupResource for --crd flag
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

type crdProcessor struct {
	apiGroupResource *restmapper.APIGroupResources
	mutex            sync.RWMutex
}

func NewCRDProcessor(resources *restmapper.APIGroupResources) *crdProcessor {
	return &crdProcessor{
		apiGroupResource: resources,
	}
}

func (p *crdProcessor) AddResourceGroup(resources *restmapper.APIGroupResources) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.apiGroupResource = resources
}

func (p *crdProcessor) GetResourceGroup() *restmapper.APIGroupResources {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.apiGroupResource
}
