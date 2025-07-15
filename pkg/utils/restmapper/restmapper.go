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
	// check that it is not a fake client
	if client != nil && !isFake {
		dc := client.GetKubeClient().Discovery()
		cachedDiscovery := memory.NewMemCacheClient(dc)
		restMapper = restmapper.NewDeferredDiscoveryRESTMapper(cachedDiscovery)
	} else {
		apiGroupResources, err := data.APIGroupResources()
		if err != nil {
			return nil, err
		}
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	}
	return restMapper, nil
}
