package admissionpolicygenerator

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/client-go/tools/cache"
)

// this file contains the handler functions for ValidatingPolicy resources.
func (c *controller) addVP(obj *policiesv1alpha1.ValidatingPolicy) {
	logger.V(2).Info("validating policy created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) updateVP(old, obj *policiesv1alpha1.ValidatingPolicy) {
	if datautils.DeepEqual(old.GetSpec(), obj.GetSpec()) {
		return
	}
	logger.V(2).Info("validating policy updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) deleteVP(obj *policiesv1alpha1.ValidatingPolicy) {
	vpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1alpha1.ValidatingPolicy)

	logger.V(2).Info("validating policy deleted", "uid", vpol.GetUID(), "kind", vpol.GetKind(), "name", vpol.GetName())
	c.enqueueVP(obj)
}

func (c *controller) enqueueVP(obj *policiesv1alpha1.ValidatingPolicy) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract policy name")
		return
	}
	c.queue.Add("ValidatingPolicy/" + key)
}
