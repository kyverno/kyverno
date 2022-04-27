package v1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type ClusterPolicyListerExpansion interface{}

//ListResources is a wrapper to List and adds the resource kind information
// as the lister is specific to a gvk we can harcode the values here
func (pl *clusterPolicyLister) ListResources(selector labels.Selector) (ret []*v1.ClusterPolicy, err error) {
	policies, err := pl.List(selector)
	for index := range policies {
		policies[index].SetGroupVersionKind(v1.SchemeGroupVersion.WithKind("ClusterPolicy"))
	}
	return policies, err
}
