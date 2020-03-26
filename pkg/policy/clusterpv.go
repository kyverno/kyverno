package policy

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/client-go/tools/cache"
)

func (pc *PolicyController) addClusterPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.ClusterPolicyViolation)
	logger := pc.log.WithValues("kind", pv.Kind, "namespace", pv.Namespace, "name", pv.Name)

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
		logger.V(4).Info("Cluster Policy Violation does not belong to an active policy, will be cleanedup")
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			logger.Error(err, "failed to delete resource")
			return
		}
		logger.V(4).Info("resource deleted")
		return
	}
	logger.V(4).Info("resource added")
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
	logger := pc.log.WithValues("kind", curPV.Kind, "namespace", curPV.Namespace, "name", curPV.Name)

	ps := pc.getPolicyForClusterPolicyViolation(curPV)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		logger.V(4).Info("Cluster Policy Violation does not belong to an active policy, will be cleanedup")
		if err := pc.pvControl.DeleteClusterPolicyViolation(curPV.Name); err != nil {
			logger.Error(err, "failed to delete resource")
			return
		}
		logger.V(4).Info("resource deleted")
		return
	}
	logger.V(4).Info("resource updated")
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

// deletePolicyViolation enqueues the Policy that manages a PolicyViolation when
// the PolicyViolation is deleted. obj could be an *kyverno.CusterPolicyViolation, or
// a DeletionFinalStateUnknown marker item.

func (pc *PolicyController) deleteClusterPolicyViolation(obj interface{}) {
	logger := pc.log
	pv, ok := obj.(*kyverno.ClusterPolicyViolation)
	// When a delete is dropped, the relist will notice a PolicyViolation in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the PolicyViolation
	// changed labels the new Policy will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.ClusterPolicyViolation)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
	}
	logger = logger.WithValues("kind", pv.Kind, "namespace", pv.Namespace, "name", pv.Name)
	ps := pc.getPolicyForClusterPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		logger.V(4).Info("Cluster Policy Violation does not belong to an active policy, will be cleanedup")
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			logger.Error(err, "failed to delete resource")
			return
		}
		logger.V(4).Info("resource deleted")
		return
	}
	logger.V(4).Info("resource updated")
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) getPolicyForClusterPolicyViolation(pv *kyverno.ClusterPolicyViolation) []*kyverno.ClusterPolicy {
	logger := pc.log.WithValues("kind", pv.Kind, "namespace", pv.Namespace, "name", pv.Name)
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
		logger.V(4).Info("user error! more than one policy is selecting policy violation", "labels", pv.Labels, "policy", policies[0].Name)
	}
	return policies
}
func (pc *PolicyController) getClusterPolicyViolationForPolicy(policy string) ([]*kyverno.ClusterPolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy)
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
