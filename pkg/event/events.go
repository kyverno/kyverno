package event

import (
	"fmt"
	"strings"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func NewPolicyFailEvent(source Source, reason Reason, engineResponse engineapi.EngineResponse, ruleResp engineapi.RuleResponse, blocked bool) Info {
	action := ResourcePassed
	if blocked {
		action = ResourceBlocked
	}
	pol := engineResponse.Policy()
	regarding := corev1.ObjectReference{
		APIVersion: pol.GetAPIVersion(),
		Kind:       pol.GetKind(),
		Name:       pol.GetName(),
		Namespace:  pol.GetNamespace(),
		UID:        pol.GetUID(),
	}
	related := engineResponse.GetResourceSpec()
	return Info{
		Regarding: regarding,
		Related: &corev1.ObjectReference{
			APIVersion: related.APIVersion,
			Kind:       related.Kind,
			Name:       related.Name,
			Namespace:  related.Namespace,
			UID:        types.UID(related.UID),
		},
		Reason:  reason,
		Source:  source,
		Message: buildPolicyEventMessage(ruleResp, engineResponse.GetResourceSpec(), blocked),
		Action:  action,
	}
}

func buildPolicyEventMessage(resp engineapi.RuleResponse, resource engineapi.ResourceSpec, blocked bool) string {
	var b strings.Builder
	if resource.Namespace != "" {
		fmt.Fprintf(&b, "%s %s/%s", resource.Kind, resource.Namespace, resource.Name)
	} else {
		fmt.Fprintf(&b, "%s %s", resource.Kind, resource.Name)
	}

	if resp.Name() != "" {
		fmt.Fprintf(&b, ": [%s] %s", resp.Name(), resp.Status())
	} else {
		fmt.Fprintf(&b, ": %s", resp.Status())
	}

	if blocked {
		fmt.Fprintf(&b, " (blocked)")
	}

	if resp.Message() != "" {
		fmt.Fprintf(&b, "; %s", resp.Message())
	}
	return b.String()
}

func NewPolicyAppliedEvent(source Source, engineResponse engineapi.EngineResponse) Info {
	resource := engineResponse.Resource
	var bldr strings.Builder
	defer bldr.Reset()

	var res string
	if resource.GetNamespace() != "" {
		res = fmt.Sprintf("%s %s/%s", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	} else {
		res = fmt.Sprintf("%s %s", resource.GetKind(), resource.GetName())
	}

	var action Action
	policy := engineResponse.Policy()
	if policy.AsMutatingPolicyLike() != nil {
		fmt.Fprintf(&bldr, "%s is successfully mutated", res)
		action = ResourceMutated
	} else {
		fmt.Fprintf(&bldr, "%s: pass", res)
		action = ResourcePassed
	}
	regarding := corev1.ObjectReference{
		APIVersion: policy.GetAPIVersion(),
		Kind:       policy.GetKind(),
		Name:       policy.GetName(),
		Namespace:  policy.GetNamespace(),
		UID:        policy.GetUID(),
	}
	related := engineResponse.GetResourceSpec()
	return Info{
		Regarding: regarding,
		Related: &corev1.ObjectReference{
			APIVersion: related.APIVersion,
			Kind:       related.Kind,
			Name:       related.Name,
			Namespace:  related.Namespace,
			UID:        types.UID(related.UID),
		},
		Reason:  PolicyApplied,
		Source:  source,
		Message: bldr.String(),
		Action:  action,
	}
}

func NewResourceViolationEvent(source Source, reason Reason, engineResponse engineapi.EngineResponse, ruleResp engineapi.RuleResponse) Info {
	var bldr strings.Builder
	defer bldr.Reset()

	pol := engineResponse.Policy()
	fmt.Fprintf(&bldr, "policy %s/%s %s: %s", pol.GetName(),
		ruleResp.Name(), ruleResp.Status(), ruleResp.Message())
	resource := engineResponse.GetResourceSpec()
	regarding := corev1.ObjectReference{
		APIVersion: resource.APIVersion,
		Kind:       resource.Kind,
		Name:       resource.Name,
		Namespace:  resource.Namespace,
		UID:        types.UID(resource.UID),
	}
	return Info{
		Regarding: regarding,
		Reason:    reason,
		Source:    source,
		Message:   bldr.String(),
		Action:    ResourcePassed,
	}
}

func NewResourceGenerationEvent(policy, rule string, source Source, resource kyverno.ResourceSpec) Info {
	msg := fmt.Sprintf("Created %s %s as a result of applying policy %s/%s", resource.GetKind(), resource.GetName(), policy, rule)
	regarding := corev1.ObjectReference{
		APIVersion: resource.APIVersion,
		Kind:       resource.Kind,
		Name:       resource.Name,
		Namespace:  resource.Namespace,
		UID:        resource.UID,
	}
	return Info{
		Regarding: regarding,
		Source:    source,
		Reason:    PolicyApplied,
		Message:   msg,
		Action:    None,
	}
}

func NewBackgroundFailedEvent(err error, policy engineapi.GenericPolicy, rule string, source Source, resource kyverno.ResourceSpec) []Info {
	events := make([]Info, 0, 1)
	regarding := corev1.ObjectReference{
		// TODO: iirc it's not safe to assume api version is set
		APIVersion: "kyverno.io/v1",
		Kind:       policy.GetKind(),
		Name:       policy.GetName(),
		Namespace:  policy.GetNamespace(),
		UID:        policy.GetUID(),
	}
	var msg string
	if rule == "" {
		msg = fmt.Sprintf("policy %s error: %v", policy.GetName(), err)
	} else {
		msg = fmt.Sprintf("policy %s/%s error: %v", policy.GetName(), rule, err)
	}
	events = append(events, Info{
		Regarding: regarding,
		Related: &corev1.ObjectReference{
			APIVersion: resource.APIVersion,
			Kind:       resource.Kind,
			Name:       resource.Name,
			Namespace:  resource.Namespace,
			UID:        resource.UID,
		},
		Source:  source,
		Reason:  PolicyError,
		Message: msg,
		Action:  None,
	})

	return events
}

func NewBackgroundSuccessEvent(source Source, policy engineapi.GenericPolicy, resources []kyverno.ResourceSpec) []Info {
	events := make([]Info, 0, len(resources))
	msg := "resource generated"
	action := ResourceGenerated
	if source == MutateExistingController {
		msg = "resource mutated"
		action = ResourceMutated
	}
	regarding := corev1.ObjectReference{
		// TODO: iirc it's not safe to assume api version is set
		APIVersion: "kyverno.io/v1",
		Kind:       policy.GetKind(),
		Name:       policy.GetName(),
		Namespace:  policy.GetNamespace(),
		UID:        policy.GetUID(),
	}
	for _, res := range resources {
		events = append(events, Info{
			Regarding: regarding,
			Related: &corev1.ObjectReference{
				APIVersion: res.APIVersion,
				Kind:       res.Kind,
				Name:       res.Name,
				Namespace:  res.Namespace,
				UID:        res.UID,
			},
			Source:  source,
			Reason:  PolicyApplied,
			Message: msg,
			Action:  action,
		})
	}

	return events
}

func NewValidatingAdmissionPolicyEvent(policy engineapi.GenericPolicy, vapName, vapBindingName string) []Info {
	regarding := corev1.ObjectReference{
		// TODO: iirc it's not safe to assume api version is set
		APIVersion: policy.GetAPIVersion(),
		Kind:       policy.GetKind(),
		Name:       policy.GetName(),
		Namespace:  policy.GetNamespace(),
		UID:        policy.GetUID(),
	}
	vapEvent := Info{
		Regarding: regarding,
		Related: &corev1.ObjectReference{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingAdmissionPolicy",
			Name:       vapName,
		},
		Source:  GeneratePolicyController,
		Action:  ResourceGenerated,
		Reason:  PolicyApplied,
		Message: fmt.Sprintf("successfully generated validating admission policy %s from policy %s", vapName, policy.GetName()),
	}
	vapBindingEvent := Info{
		Regarding: regarding,
		Related: &corev1.ObjectReference{
			APIVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingAdmissionPolicyBinding",
			Name:       vapBindingName,
		},
		Source:  GeneratePolicyController,
		Action:  ResourceGenerated,
		Reason:  PolicyApplied,
		Message: fmt.Sprintf("successfully generated validating admission policy binding %s from policy %s", vapBindingName, policy.GetName()),
	}
	return []Info{vapEvent, vapBindingEvent}
}

func NewFailedEvent(err error, policy, rule string, source Source, resource kyverno.ResourceSpec) Info {
	var msg string
	if rule == "" {
		msg = fmt.Sprintf("policy %s error: %v", policy, err)
	} else {
		msg = fmt.Sprintf("policy %s/%s error: %v", policy, rule, err)
	}
	return Info{
		Regarding: corev1.ObjectReference{
			APIVersion: resource.APIVersion,
			Kind:       resource.Kind,
			Name:       resource.Name,
			Namespace:  resource.Namespace,
			UID:        resource.UID,
		},
		Source:  source,
		Reason:  PolicyError,
		Message: msg,
		Action:  None,
	}
}

func NewDeletingPolicyEvent(policy v1beta1.DeletingPolicyLike, resource unstructured.Unstructured, err error) Info {
	regarding := corev1.ObjectReference{
		APIVersion: schema.GroupVersion(v1beta1.GroupVersion).String(),
		Kind:       policy.GetKind(),
		Name:       policy.GetName(),
		Namespace:  policy.GetNamespace(),
		UID:        policy.GetUID(),
	}
	related := &corev1.ObjectReference{
		APIVersion: resource.GetAPIVersion(),
		Kind:       resource.GetKind(),
		Namespace:  resource.GetNamespace(),
		Name:       resource.GetName(),
	}
	if err == nil {
		return Info{
			Regarding: regarding,
			Related:   related,
			Source:    CleanupController,
			Action:    ResourceCleanedUp,
			Reason:    PolicyApplied,
			Message:   fmt.Sprintf("successfully deleted the target resource %v/%v/%v", resource.GetKind(), resource.GetNamespace(), resource.GetName()),
		}
	} else {
		return Info{
			Regarding: regarding,
			Related:   related,
			Source:    CleanupController,
			Action:    None,
			Reason:    PolicyError,
			Message:   fmt.Sprintf("failed to delete the target resource %v/%v/%v: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err.Error()),
		}
	}
}

func resourceKey(resource unstructured.Unstructured) string {
	if resource.GetNamespace() != "" {
		return strings.Join([]string{resource.GetKind(), resource.GetNamespace(), resource.GetName()}, "/")
	}

	return strings.Join([]string{resource.GetKind(), resource.GetName()}, "/")
}
