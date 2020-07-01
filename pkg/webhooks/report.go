package webhooks

import (
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"

	"github.com/nirmata/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []response.EngineResponse, blocked, onUpdate bool, log logr.Logger) []event.Info {
	var events []event.Info
	// Scenario 1
	// - Admission-Response is SUCCESS && CREATE
	//   - All policies were succesfully
	//     - report event on resources
	if isResponseSuccesful(engineResponses) {
		if !onUpdate {
			// we only report events on CREATE requests
			return events
		}
		for _, er := range engineResponses {
			successRules := er.GetSuccessRules()
			successRulesStr := strings.Join(successRules, ";")
			// event on resource
			e := event.NewEvent(
				log,
				er.PolicyResponse.Resource.Kind,
				er.PolicyResponse.Resource.APIVersion,
				er.PolicyResponse.Resource.Namespace,
				er.PolicyResponse.Resource.Name,
				event.PolicyApplied.String(),
				event.AdmissionController,
				event.SRulesApply,
				successRulesStr,
				er.PolicyResponse.Policy,
			)
			events = append(events, e)
		}
		return events
	}

	// Scneario 2
	// - Admission-Response is BLOCKED
	//   - report event of policy is in enforce mode and failed to apply
	if blocked {
		for _, er := range engineResponses {
			if er.IsSuccessful() {
				// do not create event on polices that were succesfuly
				continue
			}
			if er.PolicyResponse.ValidationFailureAction != Enforce {
				// do not create event on "audit" policy
				continue
			}
			// Rules that failed
			failedRules := er.GetFailedRules()
			filedRulesStr := strings.Join(failedRules, ";")
			// Event on Policy
			e := event.NewEvent(
				log,
				"ClusterPolicy",
				kyverno.SchemeGroupVersion.String(),
				"",
				er.PolicyResponse.Policy,
				event.RequestBlocked.String(),
				event.AdmissionController,
				event.FPolicyBlockResourceUpdate,
				er.PolicyResponse.Resource.GetKey(),
				filedRulesStr,
			)
			events = append(events, e)
		}
		return events
	}

	// Scenario 3
	// - Admission-Response is SUCCESS
	//   - Some/All policies failed (policy violations generated)
	//     - report event on policy that failed
	//     - report event on resource that failed

	for _, er := range engineResponses {
		if er.IsSuccessful() {
			// do not create event on polices that were succesfuly
			continue
		}
		// Rules that failed
		failedRules := er.GetFailedRules()
		filedRulesStr := strings.Join(failedRules, ";")
		// Event on the policy
		e := event.NewEvent(
			log,
			"ClusterPolicy",
			kyverno.SchemeGroupVersion.String(),
			"",
			er.PolicyResponse.Policy,
			event.PolicyFailed.String(),
			event.AdmissionController,
			event.FPolicyApplyFailed,
			filedRulesStr,
			er.PolicyResponse.Resource.GetKey(),
		)
		events = append(events, e)
		// Event on the resource
		// event on resource
		e = event.NewEvent(
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
