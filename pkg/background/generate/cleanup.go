package generate

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *GenerateController) deleteDownstream(policy kyvernov1.PolicyInterface, ur *kyvernov1beta1.UpdateRequest) (err error) {
	if !ur.Spec.DeleteDownstream {
		return nil
	}

	// handle data policy/rule deletion
	if ur.Status.GeneratedResources != nil {
		c.log.V(4).Info("policy/rule no longer exists, deleting the downstream resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy, "rule", ur.Spec.Rule)
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

	return c.handleNonPolicyChanges(policy, ur)
}

func (c *GenerateController) handleNonPolicyChanges(policy kyvernov1.PolicyInterface, ur *kyvernov1beta1.UpdateRequest) error {
	if !ur.Spec.DeleteDownstream {
		return nil
	}

	for _, rule := range policy.GetSpec().Rules {
		if ur.Spec.Rule != rule.Name {
			continue
		}
		labels := map[string]string{
			common.GeneratePolicyLabel:          policy.GetName(),
			common.GeneratePolicyNamespaceLabel: policy.GetNamespace(),
			common.GenerateRuleLabel:            rule.Name,
			kyvernov1.LabelAppManagedBy:         kyvernov1.ValueKyvernoApp,
		}

		downstreams, err := c.getDownstreams(rule, labels, ur)
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
				c.log.V(4).Info("downstream resource deleted", spec.String())
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
			c.log.Error(err, "failed to update ur status")
		}
	}

	return nil
}

func (c *GenerateController) getDownstreams(rule kyvernov1.Rule, selector map[string]string, ur *kyvernov1beta1.UpdateRequest) (*unstructured.UnstructuredList, error) {
	gv, err := ur.Spec.GetResource().GetGroupVersion()
	if err != nil {
		return nil, err
	}

	selector[common.GenerateTriggerNameLabel] = ur.Spec.GetResource().GetName()
	selector[common.GenerateTriggerNSLabel] = ur.Spec.GetResource().GetNamespace()
	selector[common.GenerateTriggerKindLabel] = ur.Spec.GetResource().GetKind()
	selector[common.GenerateTriggerGroupLabel] = gv.Group
	selector[common.GenerateTriggerVersionLabel] = gv.Version
	if rule.Generation.GetKind() != "" {
		c.log.V(4).Info("fetching downstream resources", "APIVersion", rule.Generation.GetAPIVersion(), "kind", rule.Generation.GetKind(), "selector", selector)
		return FindDownstream(c.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), selector)
	}

	dsList := &unstructured.UnstructuredList{}
	for _, kind := range rule.Generation.CloneList.Kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		c.log.V(4).Info("fetching downstream resources", "APIVersion", apiVersion, "kind", kind, "selector", selector)
		dsList, err = FindDownstream(c.client, apiVersion, kind, selector)
		if err != nil {
			return nil, err
		} else {
			dsList.Items = append(dsList.Items, dsList.Items...)
		}
	}
	return dsList, nil
}
