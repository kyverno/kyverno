package generate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func newResourceSpec(genAPIVersion, genKind, genNamespace, genName string) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
	}
}

func TriggerFromLabels(labels map[string]string) kyvernov1.ResourceSpec {
	group := labels[common.GenerateTriggerGroupLabel]
	version := labels[common.GenerateTriggerVersionLabel]
	apiVersion := schema.GroupVersion{Group: group, Version: version}

	return kyvernov1.ResourceSpec{
		Kind:       labels[common.GenerateTriggerKindLabel],
		Namespace:  labels[common.GenerateTriggerNSLabel],
		Name:       labels[common.GenerateTriggerNameLabel],
		UID:        types.UID(labels[common.GenerateTriggerUIDLabel]),
		APIVersion: apiVersion.String(),
	}
}

func buildPolicyWithAppliedRules(policy kyvernov1.PolicyInterface, expect string) (kyvernov1.PolicyInterface, bool) {
	var rule *kyvernov1.Rule
	p := policy.CreateDeepCopy()
	for j := range p.GetSpec().Rules {
		if p.GetSpec().Rules[j].Name == expect {
			rule = &p.GetSpec().Rules[j]
			break
		}
	}
	if rule == nil {
		return nil, false
	}

	p.GetSpec().SetRules([]kyvernov1.Rule{*rule})
	return p, true
}

type triggers struct {
	logger           logr.Logger
	admissionRequest *admissionv1.AdmissionRequest
	byUID            map[types.UID]unstructured.Unstructured
}

func (c *GenerateController) collectTriggers(logger logr.Logger, admissionRequest *admissionv1.AdmissionRequest, ruleContext []kyvernov2.RuleContext) (*triggers, error) {
	resourceTypes := map[schema.GroupVersionKind]map[string]struct{}{}

	for _, rule := range ruleContext {
		gv, err := schema.ParseGroupVersion(rule.Trigger.APIVersion)
		if err != nil {
			return nil, fmt.Errorf("parse group/version from %s: %w", rule.Trigger.APIVersion, err)
		}
		gvk := gv.WithKind(rule.Trigger.Kind)
		if _, ok := resourceTypes[gvk]; !ok {
			resourceTypes[gvk] = map[string]struct{}{}
		}
		resourceTypes[gvk][rule.Trigger.Namespace] = struct{}{}
	}

	byUID := map[types.UID]unstructured.Unstructured{}

	for gvk, namespaces := range resourceTypes {
		for ns := range namespaces {
			items, err := c.client.ListResource(context.TODO(), gvk.GroupVersion().String(), gvk.Kind, ns, nil)
			if err != nil {
				return nil, fmt.Errorf("list resources for %s in namespace %q: %w", gvk.String(), ns, err)
			}

			for _, item := range items.Items {
				byUID[item.GetUID()] = item
			}
		}
	}

	return &triggers{
		logger:           logger,
		admissionRequest: admissionRequest,
		byUID:            byUID,
	}, nil
}

func (t *triggers) getTrigger(trigger kyvernov1.ResourceSpec) (unstructured.Unstructured, error) {
	t.logger.V(4).Info("fetching trigger", "trigger", trigger.String())
	if t.admissionRequest == nil {
		obj, ok := t.byUID[trigger.UID]
		if !ok {
			return unstructured.Unstructured{}, fmt.Errorf("trigger resource does not exist")
		}
		return obj, nil
	} else {
		switch t.admissionRequest.Operation {
		case admissionv1.Delete:
			return t.getTriggerForDeleteOperation(trigger)
		case admissionv1.Create:
			return t.getTriggerForCreateOperation(trigger)
		default:
			newResource, oldResource, err := admission.ExtractResources(nil, *t.admissionRequest)
			if err != nil {
				t.logger.Error(err, "failed to extract resources from admission review request")
				return unstructured.Unstructured{}, err
			}

			trigger := newResource
			if newResource.Object == nil {
				trigger = oldResource
			}
			return trigger, nil
		}
	}
}

func (t *triggers) getTriggerForDeleteOperation(trigger kyvernov1.ResourceSpec) (unstructured.Unstructured, error) {
	_, oldResource, err := admission.ExtractResources(nil, *t.admissionRequest)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to load resource from context: %w", err)
	}
	labels := oldResource.GetLabels()
	if labels[common.GeneratePolicyLabel] != "" {
		// non-trigger deletion, get trigger from ur spec
		t.logger.V(4).Info("non-trigger resource is deleted, fetching the trigger from the UR spec", "trigger", trigger.String())
		obj, ok := t.byUID[trigger.UID]
		if !ok {
			return unstructured.Unstructured{}, fmt.Errorf("trigger resource does not exist")
		}
		return obj, nil
	}
	return oldResource, nil
}

func (t *triggers) getTriggerForCreateOperation(trigger kyvernov1.ResourceSpec) (unstructured.Unstructured, error) {
	obj, ok := t.byUID[trigger.UID]
	if !ok {
		if t.admissionRequest.SubResource == "" {
			return unstructured.Unstructured{}, fmt.Errorf("trigger resource does not found")
		} else {
			t.logger.V(4).Info("trigger resource not found for subresource, reverting to resource in AdmissionReviewRequest", "subresource", t.admissionRequest.SubResource)
			newResource, _, err := admission.ExtractResources(nil, *t.admissionRequest)
			if err != nil {
				t.logger.Error(err, "failed to extract resources from admission review request")
				return unstructured.Unstructured{}, err
			}
			return newResource, nil
		}
	}
	return obj, nil
}
