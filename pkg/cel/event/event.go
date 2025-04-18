package event

import (
	"fmt"

	engine "github.com/kyverno/kyverno/pkg/cel/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type PolicyEvent struct {
	RuleName     string
	Source       EventSource
	Actions      string
	Outcome      EventOutcome
	RuleResponse *RuleResponseData
}

type EventKey struct {
	Outcome           EventOutcome
	Source            EventSource
	Actions           string
	PolicyName        string
	ResourceNamespace string
	ResourceName      string
	ResourceKind      string
	RuleName          string
}

type RuleResponseData struct {
	ResourceNamespace string
	ResourceName      string
	ResourceKind      string
	PolicyName        string
	PolicyNamespace   string
}

func formatEventMessage(event PolicyEvent) string {
	er := event.RuleResponse
	if er == nil {
		return fmt.Sprintf("%s Policy Event: Outcome=%s, Source=%s, Action=%s, Rule=%s", "Policy Engine", event.Outcome, event.Source, event.Actions, event.RuleName)
	}

	policyInfo := ""
	if er.PolicyName != "" {
		policyInfo = fmt.Sprintf("Policy: %s/%s, ", er.PolicyNamespace, er.PolicyName)
	}

	actionInfo := ""
	if len(event.Actions) > 0 {
		actionInfo = fmt.Sprintf("Actions: [%s], ", event.Actions)
	}

	return fmt.Sprintf("%s Policy Event: Outcome=%s, Source=%s, Action=%s, Rule=%s, %s%sResource: %s/%s (%s)",
		"Policy Engine",
		event.Outcome,
		event.Source,
		event.Actions,
		event.RuleName,
		policyInfo,
		actionInfo,
		er.ResourceNamespace,
		er.ResourceName,
		er.ResourceKind,
	)
}

func getKubeEventType(outcome EventOutcome) string {
	switch outcome {
	case OutcomeViolate, OutcomeError:
		return "Warning"
	case OutcomePass, OutcomeSkip:
		return "Normal"
	default:
		return "Normal"
	}
}

func getLogLevel(outcome EventOutcome) int {
	switch outcome {
	case OutcomeViolate, OutcomeError:
		return 2
	case OutcomeSkip, OutcomePass:
		return 4
	default:
		return 4
	}
}

func generateEventKey(event PolicyEvent) EventKey {
	er := event.RuleResponse
	policyName := ""
	resourceNamespace := ""
	resourceName := ""
	resourceKind := ""

	if er != nil {
		policyName = er.PolicyName
		resourceNamespace = er.ResourceNamespace
		resourceName = er.ResourceName
		resourceKind = er.ResourceKind
	}

	return EventKey{
		Outcome:           event.Outcome,
		Source:            event.Source,
		Actions:           event.Actions,
		PolicyName:        policyName,
		ResourceNamespace: resourceNamespace,
		ResourceName:      resourceName,
		ResourceKind:      resourceKind,
		RuleName:          event.RuleName,
	}
}

func NewRuleResponseDataFromEngineResponse(er *engine.EngineResponse, pr *engine.PolicyResponse, rr *engineapi.RuleResponse) *RuleResponseData {
	if er == nil || er.Resource == nil {
		return nil
	}
	erd := &RuleResponseData{
		ResourceNamespace: er.Resource.GetNamespace(),
		ResourceName:      er.Resource.GetName(),
		ResourceKind:      er.Resource.GetKind(),
	}
	erd.PolicyName = pr.Policy.GetName()
	erd.PolicyNamespace = pr.Policy.GetNamespace()
	return erd
}

func eventDetailsLogFields(event PolicyEvent) []interface{} {
	fields := []interface{}{
		"rule", event.RuleName,
		"source", event.Source,
		"actions", event.Actions,
		"outcome", event.Outcome,
	}
	if event.RuleResponse != nil {
		er := event.RuleResponse
		fields = append(fields,
			"resourceNamespace", er.ResourceNamespace,
			"resourceName", er.ResourceName,
			"resourceKind", er.ResourceKind,
			"policyName", er.PolicyName,
			"policyNamespace", er.PolicyNamespace,
		)
	}
	return fields
}
