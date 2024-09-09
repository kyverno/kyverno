package generate

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *GenerateController) deleteDownstream(policy kyvernov1.PolicyInterface, ruleContext kyvernov2.RuleContext, ur *kyvernov2.UpdateRequest) (err error) {
	// handle data policy/rule deletion
	if ur.Status.GeneratedResources != nil {
		c.log.V(4).Info("policy/rule no longer exists, deleting the downstream resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy)
		var errs []error
		failedDownstreams := []kyvernov1.ResourceSpec{}
		for _, e := range ur.Status.GeneratedResources {
			if err := c.client.DeleteResource(context.TODO(), e.GetAPIVersion(), e.GetKind(), e.GetNamespace(), e.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
				failedDownstreams = append(failedDownstreams, e)
				errs = append(errs, err)
			}
		}

		if len(errs) != 0 {
			c.log.Error(multierr.Combine(errs...), "failed to clean up downstream resources on policy deletion")
			_, err = c.statusControl.Failed(ur.GetName(),
				fmt.Sprintf("failed to clean up downstream resources on policy deletion: %v", multierr.Combine(errs...)),
				failedDownstreams)
		} else {
			_, err = c.statusControl.Success(ur.GetName(), nil)
		}
		return
	}

	if policy == nil {
		return nil
	}

	return c.handleNonPolicyChanges(policy, ruleContext, ur)
}

func (c *GenerateController) handleNonPolicyChanges(policy kyvernov1.PolicyInterface, ruleContext kyvernov2.RuleContext, ur *kyvernov2.UpdateRequest) error {
	logger := c.log.V(4).WithValues("ur", ur.Name, "policy", ur.Spec.Policy, "rule", ruleContext.Rule)
	logger.Info("synchronize for none-policy changes")
	for _, rule := range policy.GetSpec().Rules {
		if ruleContext.Rule != rule.Name {
			continue
		}
		logger.Info("deleting the downstream resource based on synchronize")
		labels := map[string]string{
			common.GeneratePolicyLabel:          policy.GetName(),
			common.GeneratePolicyNamespaceLabel: policy.GetNamespace(),
			// common.GenerateRuleLabel:            rule.Name,
			kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
		}

		downstreams, err := c.getDownstreams(rule, labels, &ruleContext)
		if err != nil {
			return fmt.Errorf("failed to fetch downstream resources: %v", err)
		}

		if len(downstreams) == 0 {
			logger.V(4).Info("no downstream resources found by label selectors", "labels", labels)
			return nil
		}
		var errs []error
		failedDownstreams := []kyvernov1.ResourceSpec{}
		for _, downstream := range downstreams {
			spec := common.ResourceSpecFromUnstructured(downstream)
			if err := c.client.DeleteResource(context.TODO(), downstream.GetAPIVersion(), downstream.GetKind(), downstream.GetNamespace(), downstream.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
				failedDownstreams = append(failedDownstreams, spec)
				errs = append(errs, err)
			} else {
				logger.Info("downstream resource deleted", "spec", spec.String())
			}
		}
		if len(errs) != 0 {
			_, err = c.statusControl.Failed(ur.GetName(),
				fmt.Sprintf("failed to clean up downstream resources on source deletion: %v", multierr.Combine(errs...)),
				failedDownstreams)
		} else {
			_, err = c.statusControl.Success(ur.GetName(), nil)
		}
		if err != nil {
			logger.Error(err, "failed to update ur status")
		}
	}

	return nil
}

func (c *GenerateController) getDownstreams(rule kyvernov1.Rule, selector map[string]string, ruleContext *kyvernov2.RuleContext) ([]unstructured.Unstructured, error) {
	gv, err := ruleContext.Trigger.GetGroupVersion()
	if err != nil {
		return nil, err
	}

	selector[common.GenerateTriggerUIDLabel] = string(ruleContext.Trigger.GetUID())
	selector[common.GenerateTriggerNSLabel] = ruleContext.Trigger.GetNamespace()
	selector[common.GenerateTriggerKindLabel] = ruleContext.Trigger.GetKind()
	selector[common.GenerateTriggerGroupLabel] = gv.Group
	selector[common.GenerateTriggerVersionLabel] = gv.Version

	for _, g := range rule.Generation.ForEachGeneration {
		return c.fetch(g.GeneratePattern, selector, ruleContext)
	}

	return c.fetch(rule.Generation.GeneratePattern, selector, ruleContext)
}

func (c *GenerateController) fetch(generatePattern kyvernov1.GeneratePattern, selector map[string]string, ruleContext *kyvernov2.RuleContext) ([]unstructured.Unstructured, error) {
	downstreamResources := []unstructured.Unstructured{}
	if generatePattern.GetKind() != "" {
		// Fetch downstream resources using trigger uid label
		c.log.V(4).Info("fetching downstream resource by the UID", "APIVersion", generatePattern.GetAPIVersion(), "kind", generatePattern.GetKind(), "selector", selector)
		dsList, err := common.FindDownstream(c.client, generatePattern.GetAPIVersion(), generatePattern.GetKind(), selector)
		if err != nil {
			return nil, err
		}

		if len(dsList.Items) == 0 {
			// Fetch downstream resources using the trigger name label
			delete(selector, common.GenerateTriggerUIDLabel)
			selector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", generatePattern.GetAPIVersion(), "kind", generatePattern.GetKind(), "selector", selector)
			dsList, err = common.FindDownstream(c.client, generatePattern.GetAPIVersion(), generatePattern.GetKind(), selector)
			if err != nil {
				return nil, err
			}
		}
		downstreamResources = append(downstreamResources, dsList.Items...)

		return downstreamResources, err
	}

	for _, kind := range generatePattern.CloneList.Kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		c.log.V(4).Info("fetching downstream cloneList resources by the UID", "APIVersion", apiVersion, "kind", kind, "selector", selector)
		dsList, err := common.FindDownstream(c.client, apiVersion, kind, selector)
		if err != nil {
			return nil, err
		}

		if len(dsList.Items) == 0 {
			delete(selector, common.GenerateTriggerUIDLabel)
			selector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", generatePattern.GetAPIVersion(), "kind", generatePattern.GetKind(), "selector", selector)
			dsList, err = common.FindDownstream(c.client, generatePattern.GetAPIVersion(), generatePattern.GetKind(), selector)
			if err != nil {
				return nil, err
			}
		}
		downstreamResources = append(downstreamResources, dsList.Items...)
	}

	return downstreamResources, nil
}
