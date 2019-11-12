package policy

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	clusterpv "github.com/nirmata/kyverno/pkg/clusterpolicyviolation"
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
	owners := clusterpv.GetOwners(dclient, *resource)
	// as we can have multiple top level owners to a resource
	// check if pv exists on each one
	// does not check for cycles
	for _, owner := range owners {
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
	resourceKey := kyverno.BuildResourceKey(kind, namespace, name)
	labelMap := map[string]string{"policy": policyName, "resource": resourceKey}
	pvSelector, err := converLabelToSelector(labelMap)
	if err != nil {
		glog.Errorf("failed to generate label sector for policy %s and resourcr %s", policyName, resourceKey)
		return kyverno.ClusterPolicyViolation{}, fmt.Errorf("failed to generate label sector for policy %s and resourcr %s", policyName, resourceKey)
	}

	pvs, err := pvLister.List(pvSelector)
	if err != nil {
		glog.Errorf("unable to list policy violations with label selector %v: %v", pvSelector, err)
		return kyverno.ClusterPolicyViolation{}, err
	}
	if len(pvs) > 1 {
		glog.Errorf("more than one policy violation exists  with labels %v", labelMap)
		return kyverno.ClusterPolicyViolation{}, fmt.Errorf("more than one policy violation exists  with labels %v", labelMap)
	}
	if len(pvs) == 0 {
		glog.V(4).Infof("policy violation does not exist with labels %v", labelMap)
		return kyverno.ClusterPolicyViolation{}, nil
	}
	// return a copy of pv
	return *pvs[0], nil
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
