package utils

import (
	"strings"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
)

// GenerateEvents generates event info for the engine responses
func GenerateEvents(engineResponses []*engineapi.EngineResponse, blocked bool) []event.Info {
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
			for i, ruleResp := range er.PolicyResponse.Rules {
				if ruleResp.Status == engineapi.RuleStatusFail || ruleResp.Status == engineapi.RuleStatusError {
					e := event.NewPolicyFailEvent(event.AdmissionController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i], blocked)
					events = append(events, e)
				}
				if !blocked {
					e := event.NewResourceViolationEvent(event.AdmissionController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i])
					events = append(events, e)
				}
			}
		} else if er.IsSkipped() { // Handle PolicyException Event
			for i, ruleResp := range er.PolicyResponse.Rules {
				isException := strings.Contains(ruleResp.Message, "rule skipped due to policy exception")
				if ruleResp.Status == engineapi.RuleStatusSkip && !blocked && isException {
					events = append(events, event.NewPolicyExceptionEvents(er, &er.PolicyResponse.Rules[i])...)
				}
			}
		} else if !er.IsSkipped() && er.Policy.GetSpec().ShouldEmitAppliedEvents() {
			e := event.NewPolicyAppliedEvent(event.AdmissionController, er)
			events = append(events, e)
		}
	}
	return events
}
