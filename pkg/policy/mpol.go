package policy

import (
	"context"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	matchutils "github.com/kyverno/kyverno/pkg/utils/match"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func (pc *policyController) handleMutateForExisting(mpol *policiesv1alpha1.MutatingPolicy) error {
	var triggers []*unstructured.Unstructured

	ur := newCELMutateUR(engineapi.NewMutatingPolicy(mpol))
	triggers = pc.getMpolTriggers(mpol.Spec.MatchConstraints)
	for _, trigger := range triggers {
		addRuleContext(ur, mpol.GetName(), common.ResourceSpecFromUnstructured(*trigger), false, false)
	}
	pc.log.V(4).Info("creating new UR for MutatingPolicy")
	// generate the UR to create the new downstream resources
	created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
	if err != nil {
		return err
	}
	if created != nil {
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (pc *policyController) getMpolTriggers(match *admissionregistrationv1alpha1.MatchResources) []*unstructured.Unstructured {
	var triggers []*unstructured.Unstructured
	objectSelector := match.ObjectSelector
	nsSelector := match.NamespaceSelector

	for _, rule := range match.ResourceRules {
		for _, group := range rule.APIGroups {
			for _, version := range rule.APIVersions {
				for _, resource := range rule.Resources {
					groupVersion := schema.GroupVersion{
						Group:   group,
						Version: version,
					}
					gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
					gvk, err := pc.restMapper.KindFor(gvr)
					if err != nil {
						pc.log.Error(err, "mapping gvr to gvk failed", "gvr", gvr)
						continue
					}
					resources, err := pc.client.ListResource(context.TODO(), groupVersion.String(), gvk.Kind, "", objectSelector)
					if err != nil {
						pc.log.Error(err, "failed to list resources", "groupVersion", groupVersion, "kind", gvk.Kind)
					}
					for i, res := range resources.Items {
						if !pc.triggerMatchesMpol(res, gvr, rule.ResourceNames, match.ExcludeResourceRules, nsSelector) {
							continue
						}
						triggers = append(triggers, &resources.Items[i])
					}
				}
			}
		}
	}
	return triggers
}

func (pc *policyController) triggerMatchesMpol(
	resource unstructured.Unstructured,
	gvr schema.GroupVersionResource,
	resourceNames []string,
	excludeRules []admissionregistrationv1alpha1.NamedRuleWithOperations,
	nsSelector *metav1.LabelSelector,
) bool {
	// check if the resource matches the excluded rules
	if len(excludeRules) > 0 {
		for _, rule := range excludeRules {
			if contains(rule.APIGroups, gvr.Group) &&
				contains(rule.APIVersions, gvr.Version) &&
				contains(rule.Resources, gvr.Resource) &&
				(len(rule.ResourceNames) == 0 || contains(rule.ResourceNames, resource.GetName())) {
				return false
			}
		}
	}

	// check if the resource matches the names specified in the rule
	if len(resourceNames) > 0 {
		if !contains(resourceNames, resource.GetName()) {
			return false
		}
	}

	// check if the resource's namespace matches the namespace selector
	if nsSelector != nil {
		nsName := resource.GetNamespace()
		namespace, err := pc.client.GetResource(context.TODO(), "v1", "Namespace", "", nsName)
		if err != nil {
			pc.log.Error(err, "failed to get namespace", "name", nsName)
			return false
		}
		isMatch, err := matchutils.CheckSelector(nsSelector, namespace.GetLabels())
		if err != nil {
			pc.log.Error(err, "failed to check namespace selector", "namespace", nsName)
			return false
		}
		if !isMatch {
			return false
		}
	}
	return true
}
