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
	//     - report failure event on policy
	//     - report failure event on resource
	//   - Some/All policies succeeded
	//     - report success event on policy
	//     - report success event on resource

	for _, er := range engineResponses {
		if !er.IsSuccessful() {
			// Rules that failed
			failedRules := er.GetFailedRules()
			failedRulesStr := strings.Join(failedRules, ";")

			// Event on the policy
			kind := "ClusterPolicy"
			if er.PolicyResponse.Policy.Namespace != "" {
				kind = "Policy"
			}
			pe := event.NewEvent(
				log,
				kind,
				kyvernov1alpha2.SchemeGroupVersion.String(),
				er.PolicyResponse.Policy.Namespace,
				er.PolicyResponse.Policy.Name,
				event.PolicyViolation.String(),
				event.AdmissionController,
				event.FPolicyApply,
				failedRulesStr,
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
				event.FResourcePolicyApply,
				failedRulesStr,
				er.PolicyResponse.Policy.Name,
			)
			events = append(events, pe, re)
		}

		if !er.IsFailed() {
			successRules := er.GetSuccessRules()
			successRulesStr := strings.Join(successRules, ";")

			// Event on the policy
			kind := "ClusterPolicy"
			if er.PolicyResponse.Policy.Namespace != "" {
				kind = "Policy"
			}
			e := event.NewEvent(
				log,
				kind,
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
