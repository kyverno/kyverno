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
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	// handle clone source deletion
	return c.deleteDownstreamForClone(policy, ur)
}

func (c *GenerateController) deleteDownstreamForClone(policy kyvernov1.PolicyInterface, ur *kyvernov1beta1.UpdateRequest) error {
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

		sources := []kyvernov1.ResourceSpec{rule.Generation.ResourceSpec}
		if rule.Generation.CloneList.Kinds != nil {
			srcs, err := c.getCloneSources(ur, rule)
			if err != nil {
				return fmt.Errorf("failed to get clone sources for the cloneList : %v", err)
			}
			sources = srcs
		}

		for _, source := range sources {
			downstreams, err := FindDownstream(c.client, source.GetAPIVersion(), source.GetKind(), labels)
			if err != nil {
				return err
			}

			var errs []error
			failedDownstreams := []kyvernov1.ResourceSpec{}
			for _, downstream := range downstreams.Items {
				if err := c.client.DeleteResource(context.TODO(), downstream.GetAPIVersion(), downstream.GetKind(), downstream.GetNamespace(), downstream.GetName(), false); err != nil && !apierrors.IsNotFound(err) {
					failedDownstreams = append(failedDownstreams, common.ResourceSpecFromUnstructured(downstream))
					errs = append(errs, err)
				}
			}
			if len(errs) != 0 {
				c.log.Error(multierr.Combine(errs...), "failed to clean up downstream resources on source deletion")
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
	}
	return nil
}

func (c *GenerateController) getCloneSources(ur *kyvernov1beta1.UpdateRequest, rule kyvernov1.Rule) (sources []kyvernov1.ResourceSpec, err error) {
	source, err := c.getTriggerForDeleteOperation(ur.Spec)
	if err != nil {
		return nil, err
	}

	labels := source.GetLabels()
	if _, ok := labels[common.GenerateTypeCloneSourceLabel]; ok {
		return []kyvernov1.ResourceSpec{newResourceSpec(source.GetAPIVersion(), source.GetKind(), source.GetNamespace(), source.GetName())}, nil
	}

	for _, kind := range rule.Generation.CloneList.Kinds {
		g, v, k, _ := kubeutils.ParseKindSelector(kind)
		sources = append(sources, newResourceSpec(schema.GroupVersion{Group: g, Version: v}.String(), k, "", ""))
	}
	return
}
