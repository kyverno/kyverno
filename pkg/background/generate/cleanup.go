package generate

import (
	"context"
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		downstreams, err := FindDownstream(c.client, rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), labels)
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
		return err
	}
	return nil
}
