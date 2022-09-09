package utils

import (
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
)

// GenerateEvents generates event info for the engine responses
func GenerateEvents(engineResponses []*response.EngineResponse, blocked bool) []event.Info {
	var events []event.Info

	//   - Some/All policies fail or error
	//     - report failure events on policy
	//     - report failure events on resource
	//   - Some/All policies succeeded
	//     - report success event on resource
	//   - Some/All policies skipped
	//     - report skipped event on resource

	for _, er := range engineResponses {
		if !er.IsSuccessful() {
			for i, ruleResp := range er.PolicyResponse.Rules {
				if ruleResp.Status == response.RuleStatusFail || ruleResp.Status == response.RuleStatusError {
					e := event.NewPolicyFailEvent(event.AdmissionController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i], blocked)
					events = append(events, e)
				}

				if !blocked {
					e := event.NewResourceViolationEvent(event.AdmissionController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i])
					events = append(events, e)
				}
			}
		} else {
			if er.IsSkipped() {
				for i := range er.PolicyResponse.Rules {
					e := event.NewPolicySkippedEvent(event.AdmissionController, event.PolicySkipped, er, &er.PolicyResponse.Rules[i])
					events = append(events, e)
				}
			} else {
				e := event.NewPolicyAppliedEvent(event.AdmissionController, er)
				events = append(events, e)
			}
		}
	}

	return events
}
