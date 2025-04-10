package event

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

func determineEventOutcomeFromRuleStatus(ruleStatus engineapi.RuleStatus) EventOutcome {
	switch ruleStatus {
	case engineapi.RuleStatusPass:
		return OutcomePass
	case engineapi.RuleStatusSkip:
		return OutcomeSkip
	case engineapi.RuleStatusFail, engineapi.RuleStatusWarn:
		return OutcomeViolate
	case engineapi.RuleStatusError:
		return OutcomeError
	default:
		return OutcomeError
	}
}

func createObjectReferenceFromRuleResponse(er *RuleResponseData) *unstructured.Unstructured {
	if er == nil {
		return nil
	}
	obj := &unstructured.Unstructured{}
	obj.SetKind(er.ResourceKind)
	obj.SetName(er.ResourceName)
	obj.SetNamespace(er.ResourceNamespace)
	return obj
}

func convertActionsToStrings(actions []admissionregistrationv1.ValidationAction) []string {
	strActions := make([]string, len(actions))
	for i, action := range actions {
		strActions[i] = string(action)
	}
	return strActions
}
