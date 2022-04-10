package webhooks

import (
	"strings"

	"github.com/go-logr/logr"
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/engine/response"

	"github.com/kyverno/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []*response.EngineResponse, blocked, onUpdate bool, log logr.Logger) []event.Info {
	var events []event.Info

	// - Admission-Response is SUCCESS
	//   - Some/All policies failed (policy violations generated)
	//     - Do not generate events on policy or resource (to avoid extra API calls)
	//   - Some/All policies succeeded
	//     - report success event on policy
	//     - report success event on resource

		for _, er := range engineResponses {
	
			if !er.IsFailed() {
				successRules := er.GetSuccessRules()
				successRulesStr := strings.Join(successRules, ";")
	
				// Event on the policy
				e := event.NewEvent(
					log,
					er.Policy.GetKind(),
					kyvernov1alpha2.SchemeGroupVersion.String(),
					er.PolicyResponse.Policy.Namespace,
					er.PolicyResponse.Policy.Name,
					event.PolicyApplied.String(),
					event.AdmissionController,
					event.SPolicyApply,
					successRulesStr,
					er.PolicyResponse.Resource.GetKey(),
				)
				events = append(events, e)
			}
		}
	return events
}
