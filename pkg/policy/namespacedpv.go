package policy

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	cache "k8s.io/client-go/tools/cache"
)

func (pc *PolicyController) addNamespacedPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.NamespacedPolicyViolation)

	if pv.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		pc.deleteNamespacedPolicyViolation(pv)
		return
	}
	// dont manage controller references as the ownerReference is assigned by violation generator

	ps := pc.getPolicyForNamespacedPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("PolicyViolation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Namespace, pv.Name); err != nil {
			glog.Errorf("Failed to deleted policy violation %s: %v", pv.Name, err)
			return
		}
		glog.V(4).Infof("PolicyViolation %s deleted", pv.Name)
		return
	}
	glog.V(4).Infof("Orphan Policy Violation %s added.", pv.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) updateNamespacedPolicyViolation(old, cur interface{}) {
	curPV := cur.(*kyverno.NamespacedPolicyViolation)
	oldPV := old.(*kyverno.NamespacedPolicyViolation)
	if curPV.ResourceVersion == oldPV.ResourceVersion {
		// Periodic resync will send update events for all known Policy Violation.
		// Two different versions of the same replica set will always have different RVs.
		return
	}

	ps := pc.getPolicyForNamespacedPolicyViolation(curPV)
	if len(ps) == 0 {
		// there is no namespaced policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("Namespaced Policy Violation %s does not belong to an active policy, will be cleanedup", curPV.Name)
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(curPV.Namespace, curPV.Name); err != nil {
			glog.Errorf("Failed to deleted namespaced policy violation %s: %v", curPV.Name, err)
			return
		}
		glog.V(4).Infof("Namespaced Policy Violation %s deleted", curPV.Name)
		return
	}
	glog.V(4).Infof("Namespaced Policy sViolation %s updated", curPV.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}

}

func (pc *PolicyController) deleteNamespacedPolicyViolation(obj interface{}) {
	pv, ok := obj.(*kyverno.NamespacedPolicyViolation)
	// When a delete is dropped, the relist will notice a PolicyViolation in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the PolicyViolation
	// changed labels the new Policy will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Infof("Couldn't get object from tombstone %#v", obj)
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.NamespacedPolicyViolation)
		if !ok {
			glog.Infof("Couldn't get object from tombstone %#v", obj)
			return
		}
	}

	ps := pc.getPolicyForNamespacedPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("Namespaced Policy Violation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Namespace, pv.Name); err != nil {
			glog.Errorf("Failed to deleted namespaced policy violation %s: %v", pv.Name, err)
			return
		}
		glog.V(4).Infof("Namespaced Policy Violation %s deleted", pv.Name)
		return
	}
	glog.V(4).Infof("Namespaced PolicyViolation %s updated", pv.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) getPolicyForNamespacedPolicyViolation(pv *kyverno.NamespacedPolicyViolation) []*kyverno.ClusterPolicy {
	policies, err := pc.pLister.GetPolicyForNamespacedPolicyViolation(pv)
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
