package webhooks

import (
	kyvernov1alpha1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1alpha1"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"

	"github.com/kyverno/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []*response.EngineResponse, blocked, onUpdate bool, log logr.Logger) []event.Info {
	var events []event.Info

	// - Admission-Response is SUCCESS
	//   - Some/All policies failed (policy violations generated)
	//     - report event on policy that failed
	//     - report event on resource that failed

	for _, er := range engineResponses {
		if er.IsSuccessful() {
			// do not create event on rules that were successful
			continue
		}
		// Rules that failed
		failedRules := er.GetFailedRules()
		filedRulesStr := strings.Join(failedRules, ";")

		// Event on the policy
		kind := "ClusterPolicy"
		if er.PolicyResponse.Policy.Namespace != "" {
			kind = "Policy"
		}
		pe := event.NewEvent(
			log,
			kind,
			kyvernov1alpha1.SchemeGroupVersion.String(),
			er.PolicyResponse.Policy.Namespace,
			er.PolicyResponse.Policy.Name,
			event.PolicyViolation.String(),
			event.AdmissionController,
			event.FPolicyApplyFailed,
			filedRulesStr,
			er.PolicyResponse.Resource.GetKey(),
		)

		// Event on the resource
		re := event.NewEvent(
			log,
			er.PolicyResponse.Resource.Kind,
			er.PolicyResponse.Resource.APIVersion,
			er.PolicyResponse.Resource.Namespace,
			er.PolicyResponse.Resource.Name,
			event.PolicyViolation.String(),
			event.AdmissionController,
			event.FResourcePolicyFailed,
			filedRulesStr,
			er.PolicyResponse.Policy.Name,
		)
		events = append(events, pe, re)
	}

	return events
}
