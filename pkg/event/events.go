package event

import (
	"fmt"
	"strings"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewPolicyFailEvent(source Source, reason Reason, engineResponse *response.EngineResponse, ruleResp *response.RuleResponse, blocked bool) *Info {
	msg := buildPolicyEventMessage(ruleResp, engineResponse.GetResourceSpec(), blocked)

	return &Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.PolicyResponse.Policy.Name,
		Namespace: engineResponse.PolicyResponse.Policy.Namespace,
		Reason:    reason.String(),
		Source:    source,
		Message:   msg,
	}
}

func buildPolicyEventMessage(resp *response.RuleResponse, resource response.ResourceSpec, blocked bool) string {
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

	if resp.Status == response.RuleStatusError && resp.Message != "" {
		fmt.Fprintf(&b, "; %s", resp.Status.String())
	}

	return b.String()
}

func getPolicyKind(policy v1.PolicyInterface) string {
	if policy.IsNamespaced() {
		return "Policy"
	}

	return "ClusterPolicy"
}

func NewPolicyAppliedEvent(source Source, engineResponse *response.EngineResponse) *Info {
	resource := engineResponse.PolicyResponse.Resource

	var msg string
	if resource.Namespace != "" {
		msg = fmt.Sprintf("%s %s/%s: pass", resource.Kind, resource.Namespace, resource.Name)
	} else {
		msg = fmt.Sprintf("%s %s: pass", resource.Kind, resource.Name)
	}

	return &Info{
		Kind:      getPolicyKind(engineResponse.Policy),
		Name:      engineResponse.PolicyResponse.Policy.Name,
		Namespace: engineResponse.PolicyResponse.Policy.Namespace,
		Reason:    PolicyApplied.String(),
		Source:    source,
		Message:   msg,
	}
}

func NewResourceViolationEvent(source Source, reason Reason, engineResponse *response.EngineResponse, ruleResp *response.RuleResponse) *Info {
	policyName := engineResponse.Policy.GetName()
	status := ruleResp.Status.String()
	msg := fmt.Sprintf("policy %s/%s %s: %s", policyName, ruleResp.Name, status, ruleResp.Message)
	resource := engineResponse.GetResourceSpec()

	return &Info{
		Kind:      resource.Kind,
		Name:      resource.Name,
		Namespace: resource.Namespace,
		Reason:    reason.String(),
		Source:    source,
		Message:   msg,
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
