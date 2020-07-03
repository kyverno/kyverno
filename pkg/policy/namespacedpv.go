package policy

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	cache "k8s.io/client-go/tools/cache"
)

func (pc *PolicyController) addNamespacedPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.PolicyViolation)
	logger := pc.log.WithValues("kind", pv.GetObjectKind(), "namespace", pv.Namespace, "name", pv.Name)

	if pv.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		pc.deleteNamespacedPolicyViolation(pv)
		return
	}

	ps := pc.getPolicyForNamespacedPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		logger.V(4).Info("namespaced policy violation does not belong to an active policy, will be cleaned up")
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Namespace, pv.Name); err != nil {
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

func (pc *PolicyController) updateNamespacedPolicyViolation(old, cur interface{}) {
	curPV := cur.(*kyverno.PolicyViolation)
	oldPV := old.(*kyverno.PolicyViolation)
	if curPV.ResourceVersion == oldPV.ResourceVersion {
		// Periodic resync will send update events for all known Policy Violation.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	logger := pc.log.WithValues("kind", curPV.Kind, "namespace", curPV.Namespace, "name", curPV.Name)

	ps := pc.getPolicyForNamespacedPolicyViolation(curPV)
	if len(ps) == 0 {
		// there is no namespaced policy for this violation, so we can delete this cluster policy violation
		logger.V(4).Info("nameapced policy violation does not belong to an active policy, will be cleanedup")
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(curPV.Namespace, curPV.Name); err != nil {
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

func (pc *PolicyController) deleteNamespacedPolicyViolation(obj interface{}) {
	logger := pc.log
	pv, ok := obj.(*kyverno.PolicyViolation)
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
		pv, ok = tombstone.Obj.(*kyverno.PolicyViolation)
		if !ok {
			logger.Info("Couldn't get object from tombstone", "obj", obj)
			return
		}
	}

	logger = logger.WithValues("kind", pv.GetObjectKind(), "namespace", pv.Namespace, "name", pv.Name)
	ps := pc.getPolicyForNamespacedPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		logger.V(4).Info("namespaced policy violation does not belong to an active policy, will be cleaned up")
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Namespace, pv.Name); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "failed to delete resource")
				return
			}
		}

		logger.V(4).Info("resource deleted")
		return
	}

	logger.V(4).Info("resource updated")
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) getPolicyForNamespacedPolicyViolation(pv *kyverno.PolicyViolation) []*kyverno.ClusterPolicy {
	logger := pc.log.WithValues("kind", pv.Kind, "namespace", pv.Namespace, "name", pv.Name)
	policies, err := pc.pLister.GetPolicyForNamespacedPolicyViolation(pv)
	if err != nil || len(policies) == 0 {
		logger.V(4).Info("get empty policy for namespaced policy violation", "reason", err.Error())
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
