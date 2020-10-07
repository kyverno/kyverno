package webhooks

import (
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"

	"github.com/kyverno/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []response.EngineResponse, blocked, onUpdate bool, log logr.Logger) []event.Info {
	var events []event.Info

	// - Admission-Response is SUCCESS
	//   - Some/All policies failed (policy violations generated)
	//     - report event on resource that failed

	for _, er := range engineResponses {
		if er.IsSuccessful() {
			// do not create event on rules that were succesful
			continue
		}
		// Rules that failed
		failedRules := er.GetFailedRules()
		filedRulesStr := strings.Join(failedRules, ";")

		// Event on the resource
		// event on resource
		e := event.NewEvent(
			log,
			er.PolicyResponse.Resource.Kind,
			er.PolicyResponse.Resource.APIVersion,
			er.PolicyResponse.Resource.Namespace,
			er.PolicyResponse.Resource.Name,
			event.PolicyViolation.String(),
			event.AdmissionController,
			event.FResourcePolicyFailed,
			filedRulesStr,
			er.PolicyResponse.Policy,
		)
		events = append(events, e)
	}

	return events
}
