package event

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewPolicyFailEvent(source Source, reason Reason, engineResponse engineapi.EngineResponse, ruleResp engineapi.RuleResponse, blocked bool) Info {
	action := ResourcePassed
	if blocked {
		action = ResourceBlocked
	}

	return Info{
		Kind:              getPolicyKind(engineResponse.Policy()),
		Name:              engineResponse.Policy().GetName(),
		Namespace:         engineResponse.Policy().GetNamespace(),
		RelatedAPIVersion: engineResponse.GetResourceSpec().APIVersion,
		RelatedKind:       engineResponse.GetResourceSpec().Kind,
		RelatedName:       engineResponse.GetResourceSpec().Name,
		RelatedNamespace:  engineResponse.GetResourceSpec().Namespace,
		Reason:            reason,
		Source:            source,
		Message:           buildPolicyEventMessage(ruleResp, engineResponse.GetResourceSpec(), blocked),
		Action:            action,
	}
}

func buildPolicyEventMessage(resp engineapi.RuleResponse, resource engineapi.ResourceSpec, blocked bool) string {
	var b strings.Builder
	if resource.Namespace != "" {
		fmt.Fprintf(&b, "%s %s/%s", resource.Kind, resource.Namespace, resource.Name)
	} else {
		fmt.Fprintf(&b, "%s %s", resource.Kind, resource.Name)
	}

	fmt.Fprintf(&b, ": [%s] %s", resp.Name(), resp.Status())
	if blocked {
		fmt.Fprintf(&b, " (blocked)")
	}

	if resp.Message() != "" {
		fmt.Fprintf(&b, "; %s", resp.Message())
	}

	return b.String()
}

func getPolicyKind(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return "Policy"
	}
	return "ClusterPolicy"
}

func getCleanupPolicyKind(policy kyvernov2alpha1.CleanupPolicyInterface) string {
	if policy.IsNamespaced() {
		return "CleanupPolicy"
	}
	return "ClusterCleanupPolicy"
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

	hasValidate := engineResponse.Policy().GetSpec().HasValidate()
	hasVerifyImages := engineResponse.Policy().GetSpec().HasVerifyImages()
	hasMutate := engineResponse.Policy().GetSpec().HasMutate()

	var action Action
	if hasValidate || hasVerifyImages {
		fmt.Fprintf(&bldr, "%s: pass", res)
		action = ResourcePassed
	} else if hasMutate {
		fmt.Fprintf(&bldr, "%s is successfully mutated", res)
		action = ResourceMutated
	}

	return Info{
		Kind:              getPolicyKind(engineResponse.Policy()),
		Name:              engineResponse.Policy().GetName(),
		Namespace:         engineResponse.Policy().GetNamespace(),
		RelatedAPIVersion: resource.GetAPIVersion(),
		RelatedKind:       resource.GetKind(),
		RelatedName:       resource.GetName(),
		RelatedNamespace:  resource.GetNamespace(),
		Reason:            PolicyApplied,
		Source:            source,
		Message:           bldr.String(),
		Action:            action,
	}
}

func NewResourceViolationEvent(source Source, reason Reason, engineResponse engineapi.EngineResponse, ruleResp engineapi.RuleResponse) Info {
	var bldr strings.Builder
	defer bldr.Reset()

	fmt.Fprintf(&bldr, "policy %s/%s %s: %s", engineResponse.Policy().GetName(),
		ruleResp.Name(), ruleResp.Status(), ruleResp.Message())
	resource := engineResponse.GetResourceSpec()

	return Info{
		Kind:      resource.Kind,
		Name:      resource.Name,
		Namespace: resource.Namespace,
		Reason:    reason,
		Source:    source,
		Message:   bldr.String(),
		Action:    ResourcePassed,
	}
}

func NewResourceGenerationEvent(policy, rule string, source Source, resource kyvernov1.ResourceSpec) Info {
	msg := fmt.Sprintf("Created %s %s as a result of applying policy %s/%s", resource.GetKind(), resource.GetName(), policy, rule)

	return Info{
		Kind:      resource.GetKind(),
		Namespace: resource.GetNamespace(),
		Name:      resource.GetName(),
		Source:    source,
		Reason:    PolicyApplied,
		Message:   msg,
		Action:    None,
	}
}

func NewBackgroundFailedEvent(err error, policy kyvernov1.PolicyInterface, rule string, source Source, resource kyvernov1.ResourceSpec) []Info {
	var events []Info
	events = append(events, Info{
		Kind:              policy.GetKind(),
		Namespace:         policy.GetNamespace(),
		Name:              policy.GetName(),
		RelatedAPIVersion: resource.GetAPIVersion(),
		RelatedKind:       resource.GetKind(),
		RelatedNamespace:  resource.GetNamespace(),
		RelatedName:       resource.GetName(),
		Source:            source,
		Reason:            PolicyError,
		Message:           fmt.Sprintf("policy %s/%s error: %v", policy.GetName(), rule, err),
		Action:            None,
	})

	return events
}

func NewBackgroundSuccessEvent(source Source, policy kyvernov1.PolicyInterface, resources []kyvernov1.ResourceSpec) []Info {
	var events []Info
	msg := "resource generated"
	action := ResourceGenerated

	if source == MutateExistingController {
		msg = "resource mutated"
		action = ResourceMutated
	}

	for _, res := range resources {
		events = append(events, Info{
			Kind:              policy.GetKind(),
			Namespace:         policy.GetNamespace(),
			Name:              policy.GetName(),
			RelatedAPIVersion: res.GetAPIVersion(),
			RelatedKind:       res.GetKind(),
			RelatedNamespace:  res.GetNamespace(),
			RelatedName:       res.GetName(),
			Source:            source,
			Reason:            PolicyApplied,
			Message:           msg,
			Action:            action,
		})
	}

	return events
}

func NewPolicyExceptionEvents(engineResponse engineapi.EngineResponse, ruleResp engineapi.RuleResponse, source Source) []Info {
	exception := ruleResp.Exception()
	exceptionName, exceptionNamespace := exception.GetName(), exception.GetNamespace()
	policyMessage := fmt.Sprintf("resource %s was skipped from rule %s due to policy exception %s/%s", resourceKey(engineResponse.PatchedResource), ruleResp.Name(), exceptionNamespace, exceptionName)
	var exceptionMessage string
	if engineResponse.Policy().GetNamespace() == "" {
		exceptionMessage = fmt.Sprintf("resource %s was skipped from policy rule %s/%s", resourceKey(engineResponse.PatchedResource), engineResponse.Policy().GetName(), ruleResp.Name())
	} else {
		exceptionMessage = fmt.Sprintf("resource %s was skipped from policy rule %s/%s/%s", resourceKey(engineResponse.PatchedResource), engineResponse.Policy().GetNamespace(), engineResponse.Policy().GetName(), ruleResp.Name())
	}
	policyEvent := Info{
		Kind:              getPolicyKind(engineResponse.Policy()),
		Name:              engineResponse.Policy().GetName(),
		Namespace:         engineResponse.Policy().GetNamespace(),
		RelatedAPIVersion: engineResponse.PatchedResource.GetAPIVersion(),
		RelatedKind:       engineResponse.PatchedResource.GetKind(),
		RelatedName:       engineResponse.PatchedResource.GetName(),
		RelatedNamespace:  engineResponse.PatchedResource.GetNamespace(),
		Reason:            PolicySkipped,
		Message:           policyMessage,
		Source:            source,
		Action:            ResourcePassed,
	}
	exceptionEvent := Info{
		Kind:              "PolicyException",
		Name:              exceptionName,
		Namespace:         exceptionNamespace,
		RelatedAPIVersion: engineResponse.PatchedResource.GetAPIVersion(),
		RelatedKind:       engineResponse.PatchedResource.GetKind(),
		RelatedName:       engineResponse.PatchedResource.GetName(),
		RelatedNamespace:  engineResponse.PatchedResource.GetNamespace(),
		Reason:            PolicySkipped,
		Message:           exceptionMessage,
		Source:            source,
		Action:            ResourcePassed,
	}
	return []Info{policyEvent, exceptionEvent}
}

func NewCleanupPolicyEvent(policy kyvernov2alpha1.CleanupPolicyInterface, resource unstructured.Unstructured, err error) Info {
	if err == nil {
		return Info{
			Kind:              getCleanupPolicyKind(policy),
			Namespace:         policy.GetNamespace(),
			Name:              policy.GetName(),
			RelatedAPIVersion: resource.GetAPIVersion(),
			RelatedKind:       resource.GetKind(),
			RelatedNamespace:  resource.GetNamespace(),
			RelatedName:       resource.GetName(),
			Source:            CleanupController,
			Action:            ResourceCleanedUp,
			Reason:            PolicyApplied,
			Message:           fmt.Sprintf("successfully cleaned up the target resource %v/%v/%v", resource.GetKind(), resource.GetNamespace(), resource.GetName()),
		}
	} else {
		return Info{
			Kind:              getCleanupPolicyKind(policy),
			Namespace:         policy.GetNamespace(),
			Name:              policy.GetName(),
			RelatedAPIVersion: resource.GetAPIVersion(),
			RelatedKind:       resource.GetKind(),
			RelatedNamespace:  resource.GetNamespace(),
			RelatedName:       resource.GetName(),
			Source:            CleanupController,
			Action:            None,
			Reason:            PolicyError,
			Message:           fmt.Sprintf("failed to clean up the target resource %v/%v/%v: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err.Error()),
		}
	}
}

func NewFailedEvent(err error, policy, rule string, source Source, resource kyvernov1.ResourceSpec) Info {
	return Info{
		Kind:      resource.GetKind(),
		Namespace: resource.GetNamespace(),
		Name:      resource.GetName(),
		Source:    source,
		Reason:    PolicyError,
		Message:   fmt.Sprintf("policy %s/%s error: %v", policy, rule, err),
		Action:    None,
	}
}

func resourceKey(resource unstructured.Unstructured) string {
	if resource.GetNamespace() != "" {
		return strings.Join([]string{resource.GetKind(), resource.GetNamespace(), resource.GetName()}, "/")
	}

	return strings.Join([]string{resource.GetKind(), resource.GetName()}, "/")
}
