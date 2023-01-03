package event

import (
	"fmt"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewPolicyFailEvent(source Source, reason Reason, engineResponse *api.EngineResponse, ruleResp *api.RuleResponse, blocked bool) Info {
	msg := buildPolicyEventMessage(ruleResp, engineResponse.GetResourceSpec(), blocked)

	return Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.PolicyResponse.Policy.Name,
		Namespace: engineResponse.PolicyResponse.Policy.Namespace,
		Reason:    reason.String(),
		Source:    source,
		Message:   msg,
	}
}

func buildPolicyEventMessage(resp *api.RuleResponse, resource api.ResourceSpec, blocked bool) string {
	var b strings.Builder
	if resource.Namespace != "" {
		fmt.Fprintf(&b, "%s %s/%s", resource.Kind, resource.Namespace, resource.Name)
	} else {
		fmt.Fprintf(&b, "%s %s", resource.Kind, resource.Name)
	}

	fmt.Fprintf(&b, ": [%s] %s", resp.Name, resp.Status.String())
	if blocked {
		fmt.Fprintf(&b, " (blocked)")
	}

	if resp.Status == api.RuleStatusError && resp.Message != "" {
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

func NewPolicyAppliedEvent(source Source, engineResponse *api.EngineResponse) Info {
	resource := engineResponse.PolicyResponse.Resource
	var bldr strings.Builder
	defer bldr.Reset()

	if resource.Namespace != "" {
		fmt.Fprintf(&bldr, "%s %s/%s: pass", resource.Kind, resource.Namespace, resource.Name)
	} else {
		fmt.Fprintf(&bldr, "%s %s: pass", resource.Kind, resource.Name)
	}

	return Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.PolicyResponse.Policy.Name,
		Namespace: engineResponse.PolicyResponse.Policy.Namespace,
		Reason:    PolicyApplied.String(),
		Source:    source,
		Message:   bldr.String(),
	}
}

func NewResourceViolationEvent(source Source, reason Reason, engineResponse *api.EngineResponse, ruleResp *api.RuleResponse) Info {
	var bldr strings.Builder
	defer bldr.Reset()

	fmt.Fprintf(&bldr, "policy %s/%s %s: %s", engineResponse.Policy.GetName(),
		ruleResp.Name, ruleResp.Status.String(), ruleResp.Message)
	resource := engineResponse.GetResourceSpec()

	return Info{
		Kind:      resource.Kind,
		Name:      resource.Name,
		Namespace: resource.Namespace,
		Reason:    reason.String(),
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
		Reason:    PolicyError.String(),
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
		Reason:    PolicyApplied.String(),
		Message:   msg,
	})

	return events
}

func NewPolicyExceptionEvent(engineResponse *api.EngineResponse, ruleResp *api.RuleResponse) Info {
	var messageBuilder strings.Builder
	defer messageBuilder.Reset()

	exceptionName, exceptionNamespace := getExceptionEventInfoFromRuleResponseMsg(ruleResp.Message)

	fmt.Fprintf(&messageBuilder, "resource %s was skipped from rule %s due to policy exception %s/%s", engineResponse.PatchedResource.GetName(), ruleResp.Name, exceptionNamespace, exceptionName)

	return Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.PolicyResponse.Policy.Name,
		Namespace: engineResponse.PolicyResponse.Policy.Namespace,
		Reason:    PolicySkipped.String(),
		Message:   messageBuilder.String(),
	}
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
