package generate

import (
	"context"
	"fmt"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

func (c *GenerateController) deleteDownstream(policy kyvernov1.PolicyInterface, ruleContext kyvernov2.RuleContext, ur *kyvernov2.UpdateRequest) error {
	// handle data policy/rule deletion
	if ur.Status.GeneratedResources != nil {
		c.log.V(4).Info("policy/rule no longer exists, deleting the downstream resource based on synchronize", "ur", ur.Name, "policy", ur.Spec.Policy)
		// Only new cleanup requests preserve the deleted policy's identity.
		// Old status records could have been assembled from forged labels.
		policyUID := types.UID(ur.GetAnnotations()[provenance.CleanupPolicyUIDAnnotation])
		if policyUID == "" {
			return nil
		}
		var deletedPolicy kyvernov1.PolicyInterface
		if namespace, name, namespaced := strings.Cut(ur.Spec.Policy, "/"); namespaced {
			deletedPolicy = &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: policyUID}}
		} else {
			deletedPolicy = &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: ur.Spec.Policy, UID: policyUID}}
		}
		var errs []error
		for _, e := range ur.Status.GeneratedResources {
			// The policy controller records verified downstream UIDs before the
			// policy disappears. Legacy name-only records cannot prove ownership.
			uid := e.GetUID()
			if uid == "" {
				continue
			}
			live, err := c.client.GetResource(context.TODO(), e.GetAPIVersion(), e.GetKind(), e.GetNamespace(), e.GetName())
			if apierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if live.GetUID() != uid {
				continue
			}
			valid, err := c.provenance.Verify(context.TODO(), deletedPolicy, live)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if !valid {
				continue
			}
			resourceVersion := live.GetResourceVersion()
			options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &resourceVersion}}
			if err := c.client.DeleteResource(context.TODO(), e.GetAPIVersion(), e.GetKind(), e.GetNamespace(), e.GetName(), false, options); err != nil && !apierrors.IsNotFound(err) {
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
		if ruleContext.Rule != rule.Name || !rule.HasGenerate() {
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

		if len(downstreams) == 0 {
			logger.V(4).Info("no downstream resources found by label selectors", "labels", labels)
			return nil
		}
		var errs []error
		for _, downstream := range downstreams {
			// Selection is only a hint, including the legacy name fallback. A
			// resource must belong to this exact policy, rule and trigger UID.
			labels := downstream.GetLabels()
			if ruleContext.Trigger.GetUID() == "" || labels[common.GenerateTriggerUIDLabel] != string(ruleContext.Trigger.GetUID()) || labels[common.GenerateRuleLabel] != rule.Name {
				continue
			}
			valid, err := c.provenance.Verify(context.TODO(), policy, &downstream)
			if err != nil {
				errs = append(errs, fmt.Errorf("verify downstream before cleanup: %w", err))
				continue
			}
			if !valid {
				continue
			}
			uid, resourceVersion := downstream.GetUID(), downstream.GetResourceVersion()
			options := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &resourceVersion}}
			spec := common.ResourceSpecFromUnstructured(downstream)
			if err := c.client.DeleteResource(context.TODO(), downstream.GetAPIVersion(), downstream.GetKind(), downstream.GetNamespace(), downstream.GetName(), false, options); err != nil && !apierrors.IsNotFound(err) {
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
