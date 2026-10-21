package admissionpolicygenerator

import (
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// this file contains the handler functions for PolicyException resources.
func (c *controller) addCELException(obj *policiesv1beta1.PolicyException) {
	logger.V(2).Info("policy exception created", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) updateCELException(old, obj *policiesv1beta1.PolicyException) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	logger.V(2).Info("policy exception updated", "uid", obj.GetUID(), "kind", obj.GetKind(), "name", obj.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) deleteCELException(obj *policiesv1beta1.PolicyException) {
	polex := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.PolicyException)

	logger.V(2).Info("policy exception deleted", "uid", polex.GetUID(), "kind", polex.GetKind(), "name", polex.GetName())
	c.enqueueCELException(obj)
}

func (c *controller) enqueueCELException(obj *policiesv1beta1.PolicyException) {
	for _, policy := range obj.Spec.PolicyRefs {
		if policy.Kind == "ValidatingPolicy" {
			vpol, err := c.getValidatingPolicy(policy.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					logger.Error(err, "unable to get validating policy from informer", "name", policy.Name)
				}
				continue
			}
			c.enqueueVP(vpol)
		} else if policy.Kind == "MutatingPolicy" {
			mpol, err := c.getMutatingPolicy(policy.Name)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					logger.Error(err, "unable to get mutating policy from informer", "name", policy.Name)
				}
				continue
			}
			c.enqueueMP(mpol)
		}
	}
	c.requeueAtExpiry(obj)
}

// requeueAtExpiry reconciles the policies an exception references again once it expires.
// getCELExceptions drops expired exceptions, but only when generation runs, and an exception
// reaching its expiresAt is not an informer event. Without this, a generated VAP/MAP would keep
// the expired exception's negated match conditions until something else touched the policy.
func (c *controller) requeueAtExpiry(obj *policiesv1beta1.PolicyException) {
	if obj.Spec.ExpiresAt == nil {
		return
	}
	delay := time.Until(obj.Spec.ExpiresAt.Time)
	if delay <= 0 {
		return
	}
	for _, policy := range obj.Spec.PolicyRefs {
		if policy.Kind == "ValidatingPolicy" || policy.Kind == "MutatingPolicy" {
			c.queue.AddAfter(policy.Kind+"/"+policy.Name, delay)
		}
	}
}
