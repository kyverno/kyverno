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
	pvs, err := getPv(pc.pvLister, pc.client, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name)
	if err != nil {
		glog.Errorf("failed to cleanUp violations: %v", err)
	}
	for _, pv := range pvs {
		if reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
			continue
		}
		glog.V(4).Infof("cleanup violations %s, on %s/%s/%s", pv.Name, pv.Spec.Kind, pv.Spec.Namespace, pv.Spec.Name)
		if err := pc.pvControl.DeletePolicyViolation(pv.Name); err != nil {
			glog.Errorf("failed to delete policy violation: %v", err)
			continue
		}
	}
}

func getPv(pvLister kyvernolister.ClusterPolicyViolationLister, client *dclient.Client, policyName, kind, namespace, name string) ([]kyverno.ClusterPolicyViolation, error) {
	var pvs []kyverno.ClusterPolicyViolation
	var err error
	// Check Violation on resource
	pv, err := getPVOnResource(pvLister, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching pv: %v", err)
		return []kyverno.ClusterPolicyViolation{}, err
	}
	if !reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
		// found a pv on resource
		pvs = append(pvs, pv)
		return pvs, nil
	}
	// Check Violations on owner
	pvs, err = getPVonOwnerRef(pvLister, client, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching pv: %v", err)
		return []kyverno.ClusterPolicyViolation{}, err
	}
	return pvs, nil
}

func getPVonOwnerRef(pvLister kyvernolister.ClusterPolicyViolationLister, dclient *dclient.Client, policyName, kind, namespace, name string) ([]kyverno.ClusterPolicyViolation, error) {
	var pvs []kyverno.ClusterPolicyViolation
	// get resource
	resource, err := dclient.GetResource(kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching the resource: %v", err)
		return pvs, err
	}
	// get owners
	// getOwners returns nil if there is any error
	owners := map[kyverno.ResourceSpec]interface{}{}
	policyviolation.GetOwner(dclient, owners, *resource)
	// as we can have multiple top level owners to a resource
	// check if pv exists on each one
	// does not check for cycles
	for owner := range owners {
		pv, err := getPVOnResource(pvLister, policyName, owner.Kind, owner.Namespace, owner.Name)
		if err != nil {
			glog.Errorf("error while fetching resource owners: %v", err)
			continue
		}
		pvs = append(pvs, pv)
	}
	return pvs, nil
}

// Wont do the claiming of objects, just lookup based on selectors and owner references
func getPVOnResource(pvLister kyvernolister.ClusterPolicyViolationLister, policyName, kind, namespace, name string) (kyverno.ClusterPolicyViolation, error) {

	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("unable to list policy violations : %v", err)
		return kyverno.ClusterPolicyViolation{}, err
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
	return kyverno.ClusterPolicyViolation{}, nil
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
