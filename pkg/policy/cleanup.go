package policy

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) cleanUpPolicyViolation(pResponse engine.PolicyResponse) {
	// 1- check if there is violation on resource (label:Selector)
	// 2- check if there is violation on owner
	//    - recursively get owner by queries the api server for owner information of the resource

	// there can be multiple violations as a resource can have multiple owners
	pvs, err := getPVs(pc.pvLister, pc.nspvLister, pc.client, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name)
	if err != nil {
		glog.Errorf("failed to cleanUp violations: %v", err)
	}

	if len(pvs) == 0 {
		return
	}

	switch pvs[0].(type) {
	case kyverno.ClusterPolicyViolation:
		for _, pv := range pvs {
			typedPV := pv.(kyverno.ClusterPolicyViolation)
			if reflect.DeepEqual(typedPV, kyverno.ClusterPolicyViolation{}) {
				continue
			}
			glog.V(4).Infof("cleanup cluster violation %s on %s", typedPV.Name, typedPV.Spec.ResourceSpec.ToKey())
			if err := pc.pvControl.DeletePolicyViolation(typedPV.Name); err != nil {
				glog.Errorf("failed to delete cluster policy violation: %v", err)
				continue
			}
		}
	case kyverno.NamespacedPolicyViolation:
		for _, pv := range pvs {
			typedPV := pv.(kyverno.NamespacedPolicyViolation)
			if reflect.DeepEqual(typedPV, kyverno.NamespacedPolicyViolation{}) {
				continue
			}
			glog.V(4).Infof("cleanup namespaced violation %s on %s", typedPV.Name, typedPV.Spec.ResourceSpec.ToKey())
			if err := pc.pvControl.DeleteNamespacedPolicyViolation(typedPV.Namespace, typedPV.Name); err != nil {
				glog.Errorf("failed to delete namespaced policy violation: %v", err)
				continue
			}
		}
	}
}

// getPVs gets clusterPolicyViolations or namespacedPolicyViolations depends on the resource scope
func getPVs(pvLister kyvernolister.ClusterPolicyViolationLister, nspvLister kyvernolister.NamespacedPolicyViolationLister,
	client *dclient.Client, policyName, kind, namespace, name string) ([]interface{}, error) {

	var pvs []interface{}
	var err error
	// Check Violation on resource
	pv, err := getPVOnResource(pvLister, nspvLister, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching violation on existing resource: %v", err)
		return nil, err
	}

	if pv != nil {
		// found a violation on resource
		pvs = append(pvs, pv)
		return pvs, nil
	}

	// Check Violations on owner
	pvs, err = getPVonOwnerRef(pvLister, nspvLister, client, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching pv: %v", err)
		return nil, err
	}
	return pvs, nil
}

func getPVonOwnerRef(pvLister kyvernolister.ClusterPolicyViolationLister, nspvLister kyvernolister.NamespacedPolicyViolationLister,
	dclient *dclient.Client, policyName, kind, namespace, name string) ([]interface{}, error) {
	var pvs []interface{}
	// get resource
	resource, err := dclient.GetResource(kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching the resource: %v", err)
		return pvs, err
	}

	// getOwners returns nil if there is any error
	owners := map[kyverno.ResourceSpec]interface{}{}
	policyviolation.GetOwner(dclient, owners, *resource)
	// as we can have multiple top level owners to a resource
	// check if pv exists on each one
	for owner := range owners {
		pv, err := getPVOnResource(pvLister, nspvLister, policyName, owner.Kind, owner.Namespace, owner.Name)
		if err != nil {
			glog.Errorf("error while fetching resource owners: %v", err)
			continue
		}
		pvs = append(pvs, pv)
	}
	return pvs, nil
}

// Wont do the claiming of objects, just lookup based on selectors and owner references
// returns cluster policy violation if resource is cluster wide, otherwise return ns pv
func getPVOnResource(pvLister kyvernolister.ClusterPolicyViolationLister, nspvLister kyvernolister.NamespacedPolicyViolationLister, policyName, kind, namespace, name string) (interface{}, error) {
	// cluster policy violation
	if namespace == "" {
		pvs, err := pvLister.List(labels.Everything())
		if err != nil {
			glog.V(2).Infof("unable to list policy violations : %v", err)
			return kyverno.ClusterPolicyViolation{}, fmt.Errorf("failed to list cluster pv: %v", err)
		}

		for _, pv := range pvs {
			// find a policy on same resource and policy combination
			if pv.Spec.Policy == policyName &&
				pv.Spec.ResourceSpec.Kind == kind &&
				pv.Spec.ResourceSpec.Namespace == namespace &&
				pv.Spec.ResourceSpec.Name == name {
				return *pv, nil
			}
		}
	}

	// namespaced policy violation
	nspvs, err := nspvLister.List(labels.Everything())
	if err != nil {
		glog.V(2).Infof("failed to list namespaced pv: %v", err)
		return kyverno.NamespacedPolicyViolation{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspv := range nspvs {
		// find a policy on same resource and policy combination
		if nspv.Spec.Policy == policyName &&
			nspv.Spec.ResourceSpec.Kind == kind &&
			nspv.Spec.ResourceSpec.Namespace == namespace &&
			nspv.Spec.ResourceSpec.Name == name {
			return *nspv, nil
		}
	}

	return nil, nil
}

func converLabelToSelector(labelMap map[string]string) (labels.Selector, error) {
	ls := &metav1.LabelSelector{}
	err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&labelMap, ls, nil)
	if err != nil {
		return nil, err
	}

	policyViolationSelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %v", err)
	}

	return policyViolationSelector, nil
}
