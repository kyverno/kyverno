package generate

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *GenerateController) deleteDownstream(policy kyvernov1.PolicyInterface, ruleContext kyvernov2.RuleContext, ur *kyvernov2.UpdateRequest) error {
	// handle data policy/rule deletion
	if ur.Status.GeneratedResources != nil {
		c.log.V(4).Info("policy/rule no longer exists, deleting the downstream resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy)
		var errs []error
		for _, e := range ur.Status.GeneratedResources {
			if err := c.client.DeleteResource(context.TODO(), e.GetAPIVersion(), e.GetKind(), e.GetNamespace(), e.GetName(), false, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, err)
			}
		}

		if len(errs) != 0 {
			combined := multierr.Combine(errs...)
			c.log.Error(combined, "failed to clean up downstream resources on policy deletion")
			return fmt.Errorf("failed to clean up downstream resources on policy deletion: %w", combined)
		}
		return nil
	}

	if policy == nil {
		return nil
	}

	return c.handleNonPolicyChanges(policy, ruleContext, ur)
}

func (c *GenerateController) handleNonPolicyChanges(policy kyvernov1.PolicyInterface, ruleContext kyvernov2.RuleContext, ur *kyvernov2.UpdateRequest) error {
	logger := c.log.V(4).WithValues("ur", ur.Name, "policy", ur.Spec.Policy, "rule", ruleContext.Rule)
	logger.Info("synchronize for non-policy changes")
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

		// When a specific clone source is deleted, scope the downstream deletion to
		// only the target(s) cloned from that source. Otherwise every target sharing
		// the same trigger would be deleted.
		if sourceUID := c.deletedCloneSourceUID(ur); sourceUID != "" {
			labels[common.GenerateSourceUIDLabel] = sourceUID
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
		for _, downstream := range downstreams {
			spec := common.ResourceSpecFromUnstructured(downstream)
			if err := c.client.DeleteResource(context.TODO(), downstream.GetAPIVersion(), downstream.GetKind(), downstream.GetNamespace(), downstream.GetName(), false, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, err)
			} else {
				logger.Info("downstream resource deleted", "spec", spec.String())
			}
		}
		if len(errs) != 0 {
			combined := multierr.Combine(errs...)
			return fmt.Errorf("failed to clean up downstream resources on source deletion: %w", combined)
		}
	}

	return nil
}

// deletedCloneSourceUID returns the UID of the clone source that triggered this
// delete request, or an empty string when the request was not caused by a clone
// source deletion (e.g. a trigger deletion). It is used to scope cloneList
// downstream cleanup to the target(s) generated from the deleted source only.
func (c *GenerateController) deletedCloneSourceUID(ur *kyvernov2.UpdateRequest) string {
	request := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	if request == nil || request.Operation != admissionv1.Delete {
		return ""
	}
	_, source, err := admissionutils.ExtractResources(nil, *request)
	if err != nil {
		c.log.Error(err, "failed to extract the deleted resource from the admission request")
		return ""
	}
	// only clone sources carry the clone-source tag; triggers do not
	if _, ok := source.GetLabels()[common.GenerateTypeCloneSourceLabel]; !ok {
		return ""
	}
	return string(source.GetUID())
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

	if len(rule.Generation.ForEachGeneration) > 0 {
		var allDownstreams []unstructured.Unstructured
		for _, g := range rule.Generation.ForEachGeneration {
			ds, err := c.fetch(g.GeneratePattern, selector, ruleContext)
			if err != nil {
				return nil, err
			}
			allDownstreams = append(allDownstreams, ds...)
		}
		return allDownstreams, nil
	}

	return c.fetch(rule.Generation.GeneratePattern, selector, ruleContext)
}

func (c *GenerateController) fetch(generatePattern kyvernov1.GeneratePattern, selector map[string]string, ruleContext *kyvernov2.RuleContext) ([]unstructured.Unstructured, error) {
	downstreamResources := []unstructured.Unstructured{}
	if generatePattern.GetKind() != "" {
		// Fetch downstream resources using trigger uid label
		c.log.V(4).Info("fetching downstream resource by the UID", "APIVersion", generatePattern.GetAPIVersion(), "kind", generatePattern.GetKind(), "selector", selector)
		dsList, err := common.FindDownstream(context.TODO(), c.client, generatePattern.GetAPIVersion(), generatePattern.GetKind(), selector)
		if err != nil {
			return nil, err
		}

		if len(dsList.Items) == 0 {
			// Fetch downstream resources using the trigger name label
			delete(selector, common.GenerateTriggerUIDLabel)
			selector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", generatePattern.GetAPIVersion(), "kind", generatePattern.GetKind(), "selector", selector)
			dsList, err = common.FindDownstream(context.TODO(), c.client, generatePattern.GetAPIVersion(), generatePattern.GetKind(), selector)
			if err != nil {
				return nil, err
			}
		}
		downstreamResources = append(downstreamResources, dsList.Items...)

		return downstreamResources, err
	}

	for _, kind := range generatePattern.CloneList.Kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		// Create a copy of selector for each iteration to prevent mutation from affecting subsequent iterations
		kindSelector := make(map[string]string, len(selector))
		for k, v := range selector {
			kindSelector[k] = v
		}
		c.log.V(4).Info("fetching downstream cloneList resources by the UID", "APIVersion", apiVersion, "kind", kind, "selector", kindSelector)
		dsList, err := common.FindDownstream(context.TODO(), c.client, apiVersion, kind, kindSelector)
		if err != nil {
			return nil, err
		}

		if len(dsList.Items) == 0 {
			delete(kindSelector, common.GenerateTriggerUIDLabel)
			kindSelector[common.GenerateTriggerNameLabel] = ruleContext.Trigger.GetName()
			c.log.V(4).Info("fetching downstream resource by the name", "APIVersion", apiVersion, "kind", kind, "selector", kindSelector)
			dsList, err = common.FindDownstream(context.TODO(), c.client, apiVersion, kind, kindSelector)
			if err != nil {
				return nil, err
			}
		}
		downstreamResources = append(downstreamResources, dsList.Items...)
	}

	return downstreamResources, nil
}
