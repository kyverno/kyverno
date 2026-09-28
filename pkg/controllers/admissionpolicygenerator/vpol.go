package admissionpolicygenerator

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/client-go/tools/cache"
)

// this file contains the handler functions for ValidatingPolicy resources.
func (c *controller) addVP(obj *policiesv1beta1.ValidatingPolicy) {
	logger.V(2).Info("validating policy created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) updateVP(old, obj *policiesv1beta1.ValidatingPolicy) {
	specChanged := !datautils.DeepEqual(old.GetSpec(), obj.GetSpec())
	// Autogen.Configs is recomputed and persisted independently by the policystatus
	// controller in reaction to the same spec change - often in a separate reconcile that
	// lands after this one. Without also reacting to that status-only update, VAP fan-out
	// (generate-vap.go) can compute its desired set from a stale Autogen.Configs and never
	// get a second chance to reconcile once policystatus catches up, leaving orphaned
	// per-group ValidatingAdmissionPolicies un-GC'd indefinitely.
	autogenChanged := !datautils.DeepEqual(old.GetStatus().Autogen, obj.GetStatus().Autogen)
	if !specChanged && !autogenChanged {
		return
	}
	logger.V(2).Info("validating policy updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) deleteVP(obj *policiesv1beta1.ValidatingPolicy) {
	vpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.ValidatingPolicy)

	logger.V(2).Info("validating policy deleted", "uid", vpol.GetUID(), "kind", vpol.GetKind(), "name", vpol.GetName())
	c.enqueueVP(obj)
}

func (c *controller) enqueueVP(obj *policiesv1beta1.ValidatingPolicy) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract policy name")
		return
	}
	c.queue.Add("ValidatingPolicy/" + key)
}
