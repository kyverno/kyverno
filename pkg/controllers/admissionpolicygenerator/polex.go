package admissionpolicygenerator

import (
	"strings"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/ext/wildcard"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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
	// Lazy-loaded: fetch ClusterPolicies at most once per invocation to avoid
	// repeated O(N) lister scans when multiple wildcard exceptions exist.
	var cpols []*kyvernov1.ClusterPolicy
	var cpolsLoaded bool
	for _, exception := range obj.Spec.Exceptions {
		// skip adding namespaced policies in the queue.
		// skip adding policies with multiple rules in the queue.
		if strings.Contains(exception.PolicyName, "/") || len(exception.RuleNames) > 1 {
			continue
		}

		if wildcard.ContainsWildcard(exception.PolicyName) {
			if !cpolsLoaded {
				var err error
				cpols, err = c.cpolLister.List(labels.Everything())
				if err != nil {
					logger.Error(err, "unable to list cluster policies for wildcard exception", "policyName", exception.PolicyName)
				}
				cpolsLoaded = true
			}
			for _, cpol := range cpols {
				if wildcard.Match(exception.PolicyName, cpol.GetName()) {
					c.enqueuePolicy(cpol)
				}
			}
			continue
		}

		cpol, err := c.getClusterPolicy(exception.PolicyName)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				logger.Error(err, "unable to get the policy from policy informer")
			}
			continue
		}
		c.enqueuePolicy(cpol)
	}
}

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
}
