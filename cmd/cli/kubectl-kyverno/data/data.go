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
var apiGroupResource *restmapper.APIGroupResources

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

func AddResourceGroup(resources *restmapper.APIGroupResources) {
	apiGroupResource = resources
}

func GetResourceGroup() *restmapper.APIGroupResources {
	return apiGroupResource
}
