package restmapper

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"
)

func GetRESTMapper(client dclient.Interface, isFake bool) (meta.RESTMapper, error) {
	var restMapper meta.RESTMapper
	var apiGroupResources []*restmapper.APIGroupResources
	// check that it is not a fake client
	if client != nil && !isFake {
		dc := client.GetKubeClient().Discovery()
		cachedDiscovery := memory.NewMemCacheClient(dc)
		restMapper = restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)
	} else {
		processor, err := data.GetProcessor()
		if err != nil {
			return nil, err
		}
		apiGroupResources1, err := processor.GetResourceGroup()
		if err != nil {
			return nil, err
		}
		if apiGroupResources1 != nil {
			apiGroupResources = append(apiGroupResources, apiGroupResources1)
		}
		apiGroupResources2, err := data.APIGroupResources()
		if err != nil {
			return nil, err
		}
		apiGroupResources = append(apiGroupResources, apiGroupResources2...)
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	}
	return restMapper, nil
}
