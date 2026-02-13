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
	var (
		restMapper        meta.RESTMapper
		apiGroupResources []*restmapper.APIGroupResources
	)
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
		processor := data.GetProcessor()
		if processor != nil {
			// there's an initialized crd processor but it wasn't passed a crd
			if crdProcessorApiGroupResources := processor.GetResourceGroup(); crdProcessorApiGroupResources != nil {
				apiGroupResources = append(apiGroupResources, crdProcessorApiGroupResources)
			}
		}

		originalApiGroupResources, err := data.APIGroupResources()
		if err != nil {
			return nil, err
		}
		apiGroupResources = append(apiGroupResources, originalApiGroupResources...)
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	}
	return restMapper, nil
}
