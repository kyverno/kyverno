package policy

import (
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cache "k8s.io/client-go/tools/cache"
)

func (pc *PolicyController) addNamespacedPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.NamespacedPolicyViolation)

	if pv.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		pc.deletePolicyViolation(pv)
		return
	}

	// generate labels to match the policy from the spec, if not present
	if updateLabels(pv) {
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(pv); controllerRef != nil {
		p := pc.resolveControllerRef(controllerRef)
		if p == nil {
			return
		}
		glog.V(4).Infof("Namespaced policy violation %s added.", pv.Name)
		pc.enqueuePolicy(p)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching Policies and sync
	// them to see if anyone wants to adopt it.
	ps := pc.getPolicyForNamespacedPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("PolicyViolation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeletePolicyViolation(pv.Name); err != nil {
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

	// generate labels to match the policy from the spec, if not present
	if updateLabels(curPV) {
		return
	}

	curControllerRef := metav1.GetControllerOf(curPV)
	oldControllerRef := metav1.GetControllerOf(oldPV)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if p := pc.resolveControllerRef(oldControllerRef); p != nil {
			pc.enqueuePolicy(p)
		}
	}
	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		p := pc.resolveControllerRef(curControllerRef)
		if p == nil {
			return
		}
		glog.V(4).Infof("PolicyViolation %s updated.", curPV.Name)
		pc.enqueuePolicy(p)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	labelChanged := !reflect.DeepEqual(curPV.Labels, oldPV.Labels)
	if labelChanged || controllerRefChanged {
		ps := pc.getPolicyForNamespacedPolicyViolation(curPV)
		if len(ps) == 0 {
			// there is no cluster policy for this violation, so we can delete this cluster policy violation
			glog.V(4).Infof("PolicyViolation %s does not belong to an active policy, will be cleanedup", curPV.Name)
			if err := pc.pvControl.DeletePolicyViolation(curPV.Name); err != nil {
				glog.Errorf("Failed to deleted policy violation %s: %v", curPV.Name, err)
				return
			}
			glog.V(4).Infof("PolicyViolation %s deleted", curPV.Name)
			return
		}
		glog.V(4).Infof("Orphan PolicyViolation %s updated", curPV.Name)
		for _, p := range ps {
			pc.enqueuePolicy(p)
		}
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
	controllerRef := metav1.GetControllerOf(pv)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	p := pc.resolveControllerRef(controllerRef)
	if p == nil {
		return
	}
	glog.V(4).Infof("PolicyViolation %s deleted", pv.Name)
	pc.enqueuePolicy(p)
}

func updateLabels(pv *kyverno.NamespacedPolicyViolation) bool {
	if pv.Spec.Policy == "" {
		glog.Error("policy not defined for violation")
		// should be cleaned up
		return false
	}

	labels := pv.GetLabels()
	newLabels := labels
	if newLabels == nil {
		newLabels = make(map[string]string)
	}

	policy, ok := newLabels["policy"]
	// key 'policy' does not present
	// or policy name has changed
	if !ok || policy != pv.Spec.Policy {
		newLabels["policy"] = pv.Spec.Policy
	}

	resource, ok := newLabels["resource"]
	// key 'resource' does not present
	// or resource defined in policy has changed
	if !ok || resource != pv.Spec.ResourceSpec.ToKey() {
		newLabels["resource"] = pv.Spec.ResourceSpec.ToKey()
	}

	if !reflect.DeepEqual(labels, newLabels) {
		pv.SetLabels(labels)
		return true
	}

	return false
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
