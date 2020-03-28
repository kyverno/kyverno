package policy

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) cleanUpPolicyViolation(pResponse response.PolicyResponse) {
	// - check if there is violation on resource (label:Selector)
	if pResponse.Resource.Namespace == "" {
		pv, err := getClusterPV(pc.cpvLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Name)
		if err != nil {
			glog.Errorf("failed to cleanUp violations: %v", err)
			return
		}

		if reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
			return
		}

		glog.V(4).Infof("cleanup cluster violation %s on %s", pv.Name, pv.Spec.ResourceSpec.ToKey())
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			glog.Errorf("failed to delete cluster policy violation %s on %s: %v", pv.Name, pv.Spec.ResourceSpec.ToKey(), err)
		}

		return
	}

	// namespace policy violation
	nspv, err := getNamespacedPV(pc.nspvLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name)
	if err != nil {
		glog.Error(err)
		return
	}

	if reflect.DeepEqual(nspv, kyverno.PolicyViolation{}) {
		return
	}
	glog.V(4).Infof("cleanup namespaced violation %s on %s.%s", nspv.Name, pResponse.Resource.Namespace, nspv.Spec.ResourceSpec.ToKey())
	if err := pc.pvControl.DeleteNamespacedPolicyViolation(nspv.Namespace, nspv.Name); err != nil {
		glog.Errorf("failed to delete namespaced policy violation %s on %s: %v", nspv.Name, nspv.Spec.ResourceSpec.ToKey(), err)
	}
}

// Wont do the claiming of objects, just lookup based on selectors
func getClusterPV(pvLister kyvernolister.ClusterPolicyViolationLister, policyName, rkind, rname string) (kyverno.ClusterPolicyViolation, error) {
	var err error
	// Check Violation on resource
	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		glog.V(2).Infof("unable to list policy violations : %v", err)
		return kyverno.ClusterPolicyViolation{}, fmt.Errorf("failed to list cluster pv: %v", err)
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == policyName &&
			pv.Spec.ResourceSpec.Kind == rkind &&
			pv.Spec.ResourceSpec.Name == rname {
			return *pv, nil
		}
	}
	return kyverno.ClusterPolicyViolation{}, nil
}

func getNamespacedPV(nspvLister kyvernolister.PolicyViolationLister, policyName, rkind, rnamespace, rname string) (kyverno.PolicyViolation, error) {
	nspvs, err := nspvLister.PolicyViolations(rnamespace).List(labels.Everything())
	if err != nil {
		glog.V(2).Infof("failed to list namespaced pv: %v", err)
		return kyverno.PolicyViolation{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspv := range nspvs {
		// find a policy on same resource and policy combination
		if nspv.Spec.Policy == policyName &&
			nspv.Spec.ResourceSpec.Kind == rkind &&
			nspv.Spec.ResourceSpec.Name == rname {
			return *nspv, nil
		}
	}

	return kyverno.PolicyViolation{}, nil
}
