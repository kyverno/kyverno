package event

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewPolicyFailEvent(source Source, reason Reason, engineResponse *engineapi.EngineResponse, ruleResp *engineapi.RuleResponse, blocked bool) Info {
	return Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.Policy.GetName(),
		Namespace: engineResponse.Policy.GetNamespace(),
		Reason:    reason,
		Source:    source,
		Message:   buildPolicyEventMessage(ruleResp, engineResponse.GetResourceSpec(), blocked),
	}
}

func buildPolicyEventMessage(resp *engineapi.RuleResponse, resource engineapi.ResourceSpec, blocked bool) string {
	var b strings.Builder
	if resource.Namespace != "" {
		fmt.Fprintf(&b, "%s %s/%s", resource.Kind, resource.Namespace, resource.Name)
	} else {
		fmt.Fprintf(&b, "%s %s", resource.Kind, resource.Name)
	}

	fmt.Fprintf(&b, ": [%s] %s", resp.Name, resp.Status)
	if blocked {
		fmt.Fprintf(&b, " (blocked)")
	}

	if resp.Status == engineapi.RuleStatusError && resp.Message != "" {
		fmt.Fprintf(&b, "; %s", resp.Message)
	}

	return b.String()
}

func getPolicyKind(policy kyvernov1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return "Policy"
	}
	return "ClusterPolicy"
}

func NewPolicyAppliedEvent(source Source, engineResponse *engineapi.EngineResponse) Info {
	resource := engineResponse.Resource
	var bldr strings.Builder
	defer bldr.Reset()

	if resource.GetNamespace() != "" {
		fmt.Fprintf(&bldr, "%s %s/%s: pass", resource.GetKind(), resource.GetNamespace(), resource.GetName())
	} else {
		fmt.Fprintf(&bldr, "%s %s: pass", resource.GetKind(), resource.GetName())
	}

	return Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.Policy.GetName(),
		Namespace: engineResponse.Policy.GetNamespace(),
		Reason:    PolicyApplied,
		Source:    source,
		Message:   bldr.String(),
	}
}

func NewResourceViolationEvent(source Source, reason Reason, engineResponse *engineapi.EngineResponse, ruleResp *engineapi.RuleResponse) Info {
	var bldr strings.Builder
	defer bldr.Reset()

	fmt.Fprintf(&bldr, "policy %s/%s %s: %s", engineResponse.Policy.GetName(),
		ruleResp.Name, ruleResp.Status, ruleResp.Message)
	resource := engineResponse.GetResourceSpec()

	return Info{
		Kind:      resource.Kind,
		Name:      resource.Name,
		Namespace: resource.Namespace,
		Reason:    reason,
		Source:    source,
		Message:   bldr.String(),
	}
}

func NewBackgroundFailedEvent(err error, policy, rule string, source Source, r *unstructured.Unstructured) []Info {
	if r == nil {
		return nil
	}

	var events []Info
	events = append(events, Info{
		Kind:      r.GetKind(),
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
		Source:    source,
		Reason:    PolicyError,
		Message:   fmt.Sprintf("policy %s/%s error: %v", policy, rule, err),
	})

	return events
}

func NewBackgroundSuccessEvent(policy, rule string, source Source, r *unstructured.Unstructured) []Info {
	if r == nil {
		return nil
	}

	var events []Info
	msg := fmt.Sprintf("policy %s/%s applied", policy, rule)
	events = append(events, Info{
		Kind:      r.GetKind(),
		Namespace: r.GetNamespace(),
		Name:      r.GetName(),
		Source:    source,
		Reason:    PolicyApplied,
		Message:   msg,
	})

	return events
}

func NewPolicyExceptionEvents(engineResponse *engineapi.EngineResponse, ruleResp *engineapi.RuleResponse) []Info {
	exceptionName, exceptionNamespace := getExceptionEventInfoFromRuleResponseMsg(ruleResp.Message)
	policyMessage := fmt.Sprintf("resource %s was skipped from rule %s due to policy exception %s/%s", engineResponse.PatchedResource.GetName(), ruleResp.Name, exceptionNamespace, exceptionName)
	var exceptionMessage string
	if engineResponse.Policy.GetNamespace() == "" {
		exceptionMessage = fmt.Sprintf("resource %s was skipped from policy rule %s/%s", engineResponse.PatchedResource.GetName(), engineResponse.Policy.GetName(), ruleResp.Name)
	} else {
		exceptionMessage = fmt.Sprintf("resource %s was skipped from policy rule %s/%s/%s", engineResponse.PatchedResource.GetName(), engineResponse.Policy.GetNamespace(), engineResponse.Policy.GetName(), ruleResp.Name)
	}
	policyEvent := Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.Policy.GetName(),
		Namespace: engineResponse.Policy.GetNamespace(),
		Reason:    PolicySkipped,
		Message:   policyMessage,
	}
	exceptionEvent := Info{
		Kind:      "PolicyException",
		Name:      exceptionName,
		Namespace: exceptionNamespace,
		Reason:    PolicySkipped,
		Message:   exceptionMessage,
	}
	return []Info{policyEvent, exceptionEvent}
}

func getExceptionEventInfoFromRuleResponseMsg(message string) (name string, namespace string) {
	key := message[strings.LastIndex(message, " ")+1:]
	arr := strings.Split(key, "/")

	if len(arr) > 1 {
		namespace = arr[0]
		name = arr[1]
	} else {
		namespace = ""
		name = arr[0]
	}

	return name, namespace
}
