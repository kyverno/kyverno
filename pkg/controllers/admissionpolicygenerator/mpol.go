package admissionpolicygenerator

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/client-go/tools/cache"
)

// this file contains the handler functions for MutatingPolicy resources.
func (c *controller) addMP(obj *policiesv1alpha1.MutatingPolicy) {
	logger.V(2).Info("mutating policy created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueMP(obj)
}

func (c *controller) updateMP(old, obj *policiesv1alpha1.MutatingPolicy) {
	if datautils.DeepEqual(old.GetSpec(), obj.GetSpec()) {
		return
	}
	logger.V(2).Info("mutating policy updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueMP(obj)
}

func (c *controller) deleteMP(obj *policiesv1alpha1.MutatingPolicy) {
	mpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1alpha1.MutatingPolicy)

	logger.V(2).Info("mutating policy deleted", "uid", mpol.GetUID(), "kind", mpol.GetKind(), "name", mpol.GetName())
	c.enqueueMP(obj)
}

func (c *controller) enqueueMP(obj *policiesv1alpha1.MutatingPolicy) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract policy name")
		return
	}
	c.queue.Add("MutatingPolicy/" + key)
}
