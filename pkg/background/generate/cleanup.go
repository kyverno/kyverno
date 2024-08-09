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
			common.GenerateRuleLabel:            rule.Name,
			kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
		}

		downstreams, err := c.getDownstreams(rule, labels, &ruleContext)
		if err != nil {
			return fmt.Errorf("failed to fetch downstream resources: %v", err)
		}
		var errs []error
		failedDownstreams := []kyvernov1.ResourceSpec{}
		for _, downstream := range downstreams.Items {
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

func (c *GenerateController) getDownstreams(rule kyvernov1.Rule, selector map[string]string, ruleContext *kyvernov2.RuleContext) (*unstructured.UnstructuredList, error) {
	gv, err := ruleContext.Trigger.GetGroupVersion()
	if err != nil {
		return nil, err
	}

	selector[common.GenerateTriggerUIDLabel] = string(ruleContext.Trigger.GetUID())
	selector[common.GenerateTriggerNSLabel] = ruleContext.Trigger.GetNamespace()
	selector[common.GenerateTriggerKindLabel] = ruleContext.Trigger.GetKind()
	selector[common.GenerateTriggerGroupLabel] = gv.Group
	selector[common.GenerateTriggerVersionLabel] = gv.Version
	if rule.Generation.GetKind() != "" {
		// Fetch downstream resources using trigger uid label
		c.log.V(4).Info("fetching downstream resource by the UID", "APIVersion", rule.Generation.GetAPIVersion(), "kind", rule.Generation.GetKind(), "selector", selector)
		downstreamList, err := common.FindDownstream(c.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), selector)
		if err != nil {
			return nil, err
		}

		if len(downstreamList.Items) == 0 {
			// Fetch downstream resources using the trigger name label
			delete(selector, common.GenerateTriggerUIDLabel)
			selector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", rule.Generation.GetAPIVersion(), "kind", rule.Generation.GetKind(), "selector", selector)
			dsList, err := common.FindDownstream(c.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), selector)
			if err != nil {
				return nil, err
			}
			downstreamList.Items = append(downstreamList.Items, dsList.Items...)
		}

		return downstreamList, err
	}

	dsList := &unstructured.UnstructuredList{}
	for _, kind := range rule.Generation.CloneList.Kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		c.log.V(4).Info("fetching downstream cloneList resources by the UID", "APIVersion", apiVersion, "kind", kind, "selector", selector)
		dsList, err = common.FindDownstream(c.client, apiVersion, kind, selector)
		if err != nil {
			return nil, err
		}

		if len(dsList.Items) == 0 {
			delete(selector, common.GenerateTriggerUIDLabel)
			selector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", rule.Generation.GetAPIVersion(), "kind", rule.Generation.GetKind(), "selector", selector)
			dsList, err = common.FindDownstream(c.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), selector)
			if err != nil {
				return nil, err
			}
		}
	}
	return dsList, nil
}
