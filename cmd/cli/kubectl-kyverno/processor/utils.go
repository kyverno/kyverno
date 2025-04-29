package processor

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/restmapper"
)

func policyHasValidateOrVerifyImageChecks(policy kyvernov1.PolicyInterface) bool {
	for _, rule := range policy.GetSpec().Rules {
		//  engine.validate handles both validate and verifyImageChecks atm
		if rule.HasValidate() || rule.HasVerifyImageChecks() {
			return true
		}
	}
	return false
}

func getRESTMapper(client dclient.Interface, isFake bool) (meta.RESTMapper, error) {
	var restMapper meta.RESTMapper
	// check that it is not a fake client
	if client != nil && !isFake {
		apiGroupResources, err := restmapper.GetAPIGroupResources(client.GetKubeClient().Discovery())
		if err != nil {
			return nil, err
		}
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	} else {
		apiGroupResources, err := data.APIGroupResources()
		if err != nil {
			return nil, err
		}
		restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	}
	return restMapper, nil
}
