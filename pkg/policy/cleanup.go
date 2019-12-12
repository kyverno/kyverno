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
	if pResponse.Resource.Namespace == "" {
		pvs, err := getClusterPVs(pc.pvLister, pc.client, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Name)
		if err != nil {
			glog.Errorf("failed to cleanUp violations: %v", err)
			return
		}

		for _, pv := range pvs {
			if reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
				continue
			}
			glog.V(4).Infof("cleanup cluster violation %s on %s", pv.Name, pv.Spec.ResourceSpec.ToKey())
			if err := pc.pvControl.DeletePolicyViolation(pv.Name); err != nil {
				glog.Errorf("failed to delete cluster policy violation %s on %s: %v", pv.Name, pv.Spec.ResourceSpec.ToKey(), err)
				continue
			}
		}
		return
	}

	nspvs, err := getNamespacedPVs(pc.nspvLister, pc.client, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name)
	if err != nil {
		glog.Error(err)
		return
	}

	for _, pv := range nspvs {
		if reflect.DeepEqual(pv, kyverno.PolicyViolation{}) {
			continue
		}
		glog.V(4).Infof("cleanup namespaced violation %s on %s", pv.Name, pv.Spec.ResourceSpec.ToKey())
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Namespace, pv.Name); err != nil {
			glog.Errorf("failed to delete namespaced policy violation %s on %s: %v", pv.Name, pv.Spec.ResourceSpec.ToKey(), err)
			continue
		}
	}
}

func getClusterPVs(pvLister kyvernolister.ClusterPolicyViolationLister, client *dclient.Client, policyName, kind, name string) ([]kyverno.ClusterPolicyViolation, error) {
	var pvs []kyverno.ClusterPolicyViolation
	var err error
	// Check Violation on resource
	pv, err := getClusterPVOnResource(pvLister, policyName, kind, name)
	if err != nil {
		glog.V(4).Infof("error while fetching violation on existing resource: %v", err)
		return nil, err
	}

	if !reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
		// found a violation on resource
		pvs = append(pvs, pv)
		return pvs, nil
	}

	// Check Violations on owner
	pvs, err = getClusterPVonOwnerRef(pvLister, client, policyName, kind, name)
	if err != nil {
		glog.V(4).Infof("error while fetching pv: %v", err)
		return nil, err
	}
	return pvs, nil
}

// Wont do the claiming of objects, just lookup based on selectors and owner references
func getClusterPVOnResource(pvLister kyvernolister.ClusterPolicyViolationLister, policyName, kind, name string) (kyverno.ClusterPolicyViolation, error) {
	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		glog.V(2).Infof("unable to list policy violations : %v", err)
		return kyverno.ClusterPolicyViolation{}, fmt.Errorf("failed to list cluster pv: %v", err)
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == policyName &&
			pv.Spec.ResourceSpec.Kind == kind &&
			pv.Spec.ResourceSpec.Name == name {
			return *pv, nil
		}
	}
	return kyverno.ClusterPolicyViolation{}, nil
}

func getClusterPVonOwnerRef(pvLister kyvernolister.ClusterPolicyViolationLister, dclient *dclient.Client, policyName, kind, name string) ([]kyverno.ClusterPolicyViolation, error) {
	var pvs []kyverno.ClusterPolicyViolation
	// get resource
	resource, err := dclient.GetResource(kind, "", name)
	if err != nil {
		glog.V(4).Infof("error while fetching the resource: %v", err)
		return pvs, fmt.Errorf("error while fetching the resource: %v", err)
	}

	// getOwners returns nil if there is any error
	owners := map[kyverno.ResourceSpec]interface{}{}
	policyviolation.GetOwner(dclient, owners, *resource)
	// as we can have multiple top level owners to a resource
	// check if pv exists on each one
	for owner := range owners {
		pv, err := getClusterPVOnResource(pvLister, policyName, owner.Kind, owner.Name)
		if err != nil {
			glog.Errorf("error while fetching resource owners: %v", err)
			continue
		}
		pvs = append(pvs, pv)
	}
	return pvs, nil
}

func getNamespacedPVs(nspvLister kyvernolister.PolicyViolationLister, client *dclient.Client, policyName, kind, namespace, name string) ([]kyverno.PolicyViolation, error) {
	var pvs []kyverno.PolicyViolation
	var err error
	pv, err := getNamespacedPVOnResource(nspvLister, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching violation on existing resource: %v", err)
		return nil, err
	}

	if !reflect.DeepEqual(pv, kyverno.PolicyViolation{}) {
		// found a violation on resource
		pvs = append(pvs, pv)
		return pvs, nil
	}

	// Check Violations on owner
	pvs, err = getNamespacedPVonOwnerRef(nspvLister, client, policyName, kind, namespace, name)
	if err != nil {
		glog.V(4).Infof("error while fetching pv: %v", err)
		return nil, err
	}
	return pvs, nil
}

func getNamespacedPVOnResource(nspvLister kyvernolister.PolicyViolationLister, policyName, kind, namespace, name string) (kyverno.PolicyViolation, error) {
	nspvs, err := nspvLister.PolicyViolations(namespace).List(labels.Everything())
	if err != nil {
		glog.V(2).Infof("failed to list namespaced pv: %v", err)
		return kyverno.PolicyViolation{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspv := range nspvs {
		// find a policy on same resource and policy combination
		if nspv.Spec.Policy == policyName &&
			nspv.Spec.ResourceSpec.Kind == kind &&
			nspv.Spec.ResourceSpec.Name == name {
			return *nspv, nil
		}
	}
	return kyverno.PolicyViolation{}, nil
}

func getNamespacedPVonOwnerRef(nspvLister kyvernolister.PolicyViolationLister, dclient *dclient.Client, policyName, kind, namespace, name string) ([]kyverno.PolicyViolation, error) {
	var pvs []kyverno.PolicyViolation
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
		pv, err := getNamespacedPVOnResource(nspvLister, policyName, owner.Kind, namespace, owner.Name)
		if err != nil {
			glog.Errorf("error while fetching resource owners: %v", err)
			continue
		}
		pvs = append(pvs, pv)
	}
	return pvs, nil
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
