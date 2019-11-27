package policy

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/client-go/tools/cache"
)

func (pc *PolicyController) addClusterPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.ClusterPolicyViolation)

	if pv.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		pc.deleteClusterPolicyViolation(pv)
		return
	}
	// dont manage controller references as the ownerReference is assigned by violation generator

	ps := pc.getPolicyForClusterPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("Cluster Policy Violation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			glog.Errorf("Failed to deleted cluster policy violation %s: %v", pv.Name, err)
			return
		}
		glog.V(4).Infof("Cluster Policy Violation %s deleted", pv.Name)
		return
	}
	glog.V(4).Infof("Cluster Policy Violation %s added.", pv.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) updateClusterPolicyViolation(old, cur interface{}) {
	curPV := cur.(*kyverno.ClusterPolicyViolation)
	oldPV := old.(*kyverno.ClusterPolicyViolation)
	if curPV.ResourceVersion == oldPV.ResourceVersion {
		// Periodic resync will send update events for all known Policy Violation.
		// Two different versions of the same replica set will always have different RVs.
		return
	}

	ps := pc.getPolicyForClusterPolicyViolation(curPV)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("Cluster Policy Violation %s does not belong to an active policy, will be cleanedup", curPV.Name)
		if err := pc.pvControl.DeleteClusterPolicyViolation(curPV.Name); err != nil {
			glog.Errorf("Failed to deleted cluster policy violation %s: %v", curPV.Name, err)
			return
		}
		glog.V(4).Infof("PolicyViolation %s deleted", curPV.Name)
		return
	}
	glog.V(4).Infof("Cluster PolicyViolation %s updated", curPV.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

// deletePolicyViolation enqueues the Policy that manages a PolicyViolation when
// the PolicyViolation is deleted. obj could be an *kyverno.CusterPolicyViolation, or
// a DeletionFinalStateUnknown marker item.

func (pc *PolicyController) deleteClusterPolicyViolation(obj interface{}) {
	pv, ok := obj.(*kyverno.ClusterPolicyViolation)
	// When a delete is dropped, the relist will notice a PolicyViolation in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the PolicyViolation
	// changed labels the new Policy will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.ClusterPolicyViolation)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
	}
	ps := pc.getPolicyForClusterPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("Cluster Policy Violation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			glog.Errorf("Failed to deleted cluster policy violation %s: %v", pv.Name, err)
			return
		}
		glog.V(4).Infof("Cluster Policy Violation %s deleted", pv.Name)
		return
	}
	glog.V(4).Infof("Cluster PolicyViolation %s updated", pv.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) getPolicyForClusterPolicyViolation(pv *kyverno.ClusterPolicyViolation) []*kyverno.ClusterPolicy {
	policies, err := pc.pLister.GetPolicyForPolicyViolation(pv)
	if err != nil || len(policies) == 0 {
		return nil
	}
	// Because all PolicyViolations's belonging to a Policy should have a unique label key,
	// there should never be more than one Policy returned by the above method.
	// If that happens we should probably dynamically repair the situation by ultimately
	// trying to clean up one of the controllers, for now we just return the older one
	if len(policies) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		glog.V(4).Infof("user error! more than one policy is selecting policy violation %s with labels: %#v, returning %s",
			pv.Name, pv.Labels, policies[0].Name)
	}
	return policies
}
func (pc *PolicyController) getClusterPolicyViolationForPolicy(policy *kyverno.ClusterPolicy) ([]*kyverno.ClusterPolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy.Name)
	if err != nil {
		return nil, err
	}
	// Get List of cluster policy violation
	cpvList, err := pc.cpvLister.List(policySelector)
	if err != nil {
		return nil, err
	}
	return cpvList, nil
}
