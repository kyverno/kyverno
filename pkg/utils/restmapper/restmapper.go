package restmapper

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery/cached/memory"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
)

func GetRESTMapper(client dclient.Interface) (meta.RESTMapper, error) {
	var restMapper meta.RESTMapper

	// check that it is not a fake client
	isFake := false
	if client != nil {
		if kc := client.GetKubeClient(); kc == nil {
			isFake = true
		} else if _, ok := kc.(*kubefake.Clientset); ok {
			isFake = true
		}
	}

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
