package admissionpolicygenerator

import (
	"strings"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// this file contains the handler functions for PolicyException resources.
func (c *controller) addException(obj *kyvernov2.PolicyException) {
	logger.V(2).Info("policy exception created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueException(obj)
}

func (c *controller) updateException(old, obj *kyvernov2.PolicyException) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.V(2).Info("policy exception updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueException(obj)
}

func (c *controller) deleteException(obj *kyvernov2.PolicyException) {
	polex := kubeutils.GetObjectWithTombstone(obj).(*kyvernov2.PolicyException)

	logger.V(2).Info("policy exception deleted", "uid", polex.GetUID(), "kind", polex.GetKind(), "name", polex.GetName())
	c.enqueueException(obj)
}

func (c *controller) enqueueException(obj *kyvernov2.PolicyException) {
	for _, exception := range obj.Spec.Exceptions {
		// skip adding namespaced policies in the queue.
		// skip adding policies with multiple rules in the queue.
		if strings.Contains(exception.PolicyName, "/") || len(exception.RuleNames) > 1 {
			continue
		}

		cpol, err := c.getClusterPolicy(exception.PolicyName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return
			}
			logger.Error(err, "unable to get the policy from policy informer")
			return
		}
		c.enqueuePolicy(cpol)
	}
}

func (c *controller) addCELException(obj *policiesv1alpha1.PolicyException) {
	logger.V(2).Info("policy exception created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) updateCELException(old, obj *policiesv1alpha1.PolicyException) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.V(2).Info("policy exception updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) deleteCELException(obj *policiesv1alpha1.PolicyException) {
	polex := kubeutils.GetObjectWithTombstone(obj).(*policiesv1alpha1.PolicyException)

	logger.V(2).Info("policy exception deleted", "uid", polex.GetUID(), "kind", polex.GetKind(), "name", polex.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) enqueueCELException(obj *policiesv1alpha1.PolicyException) {
	for _, policy := range obj.Spec.PolicyRefs {
		if policy.Kind == "ValidatingPolicy" {
			vpol, err := c.getValidatingPolicy(policy.Name)
			if err != nil {
				return
			}
			c.enqueueVP(vpol)
		} else if policy.Kind == "MutatingPolicy" {
			mpol, err := c.getMutatingPolicy(policy.Name)
			if err != nil {
				return
			}
			c.enqueueMP(mpol)
		}
	}
}
