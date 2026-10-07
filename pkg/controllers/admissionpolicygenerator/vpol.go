package admissionpolicygenerator

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"k8s.io/client-go/tools/cache"
)

// this file contains the handler functions for ValidatingPolicy and NamespacedValidatingPolicy resources.
func (c *controller) addVP(obj policiesv1beta1.ValidatingPolicyLike) {
	logger.V(2).Info("validating policy created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) updateVP(old, obj policiesv1beta1.ValidatingPolicyLike) {
	if datautils.DeepEqual(old.GetSpec(), obj.GetSpec()) {
		return
	}
	logger.V(2).Info("validating policy updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueVP(obj)
}

func (c *controller) deleteVP(obj policiesv1beta1.ValidatingPolicyLike) {
	vpol := kubeutils.GetObjectWithTombstone(obj).(policiesv1beta1.ValidatingPolicyLike)

	logger.V(2).Info("validating policy deleted", "uid", vpol.GetUID(), "kind", vpol.GetKind(), "name", vpol.GetName())
	c.enqueueVP(obj)
}

func (c *controller) enqueueVP(obj policiesv1beta1.ValidatingPolicyLike) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract policy name")
		return
	}
	if obj.GetNamespace() != "" {
		// the worker would reject a three-part key, so pass it through unparsed
		c.queue.Add(cache.ExplicitKey("NamespacedValidatingPolicy/" + key))
	} else {
		c.queue.Add("ValidatingPolicy/" + key)
	}
}

// parseNamespacedPolicyKey extracts the namespace and name from a NamespacedValidatingPolicy/<namespace>/<name> key.
func parseNamespacedPolicyKey(key string) (string, string, bool) {
	parts := strings.SplitN(key, "/", 3)
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
