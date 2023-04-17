package utils

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
)

// GenerateEvents generates event info for the engine responses
func GenerateEvents(engineResponses []engineapi.EngineResponse, blocked bool) []event.Info {
	var events []event.Info
	//   - Some/All policies fail or error
	//     - report failure events on policy
	//     - report failure events on resource
	//   - Some/All policies succeeded
	//     - report success event on resource
	//   - Some/All policies skipped
	//     - report skipped event on resource
	for _, er := range engineResponses {
		if er.IsEmpty() {
			continue
		}
		if !er.IsSuccessful() {
			for _, ruleResp := range er.PolicyResponse.Rules {
				if ruleResp.Status() == engineapi.RuleStatusFail || ruleResp.Status() == engineapi.RuleStatusError {
					e := event.NewPolicyFailEvent(event.AdmissionController, event.PolicyViolation, er, ruleResp, blocked)
					events = append(events, e)
				}
				if !blocked {
					e := event.NewResourceViolationEvent(event.AdmissionController, event.PolicyViolation, er, ruleResp)
					events = append(events, e)
				}
			}
		} else if er.IsSkipped() { // Handle PolicyException Event
			for _, ruleResp := range er.PolicyResponse.Rules {
				if ruleResp.Status() == engineapi.RuleStatusSkip && !blocked && ruleResp.IsException() {
					events = append(events, event.NewPolicyExceptionEvents(er, ruleResp, event.AdmissionController)...)
				}
			}
		} else if !er.IsSkipped() {
			e := event.NewPolicyAppliedEvent(event.AdmissionController, er)
			events = append(events, e)
		}
	}
	return events
}
