package webhooks

import (
	"strings"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []response.EngineResponse, onUpdate bool) []event.Info {
	var events []event.Info
	if !isResponseSuccesful(engineResponses) {
		for _, er := range engineResponses {
			if er.IsSuccesful() {
				// dont create events on success
				continue
			}
			// default behavior is audit
			reason := event.PolicyViolation
			if er.PolicyResponse.ValidationFailureAction == Enforce {
				reason = event.RequestBlocked
			}
			failedRules := er.GetFailedRules()
			filedRulesStr := strings.Join(failedRules, ";")
			if onUpdate {
				var e event.Info
				// UPDATE
				// event on resource
				e = event.NewEvent(
					er.PolicyResponse.Resource.Kind,
					er.PolicyResponse.Resource.APIVersion,
					er.PolicyResponse.Resource.Namespace,
					er.PolicyResponse.Resource.Name,
					reason.String(),
					event.AdmissionController,
					event.FPolicyApplyBlockUpdate,
					filedRulesStr,
					er.PolicyResponse.Policy,
				)
				glog.V(4).Infof("UPDATE event on resource %s/%s/%s with policy %s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name, er.PolicyResponse.Policy)
				events = append(events, e)

				// event on policy
				e = event.NewEvent(
					"ClusterPolicy",
					kyverno.SchemeGroupVersion.String(),
					"",
					er.PolicyResponse.Policy,
					reason.String(),
					event.AdmissionController,
					event.FPolicyBlockResourceUpdate,
					er.PolicyResponse.Resource.GetKey(),
					filedRulesStr,
				)
				glog.V(4).Infof("UPDATE event on policy %s", er.PolicyResponse.Policy)
				events = append(events, e)

			} else {
				// CREATE
				// event on policy
				e := event.NewEvent(
					"ClusterPolicy",
					kyverno.SchemeGroupVersion.String(),
					"",
					er.PolicyResponse.Policy,
					reason.String(),
					event.AdmissionController,
					event.FPolicyApplyBlockCreate,
					er.PolicyResponse.Resource.GetKey(),
					filedRulesStr,
				)
				glog.V(4).Infof("CREATE event on policy %s", er.PolicyResponse.Policy)
				events = append(events, e)
			}
		}
		return events
	}
	if !onUpdate {
		// All policies were applied succesfully
		// CREATE
		for _, er := range engineResponses {
			successRules := er.GetSuccessRules()
			successRulesStr := strings.Join(successRules, ";")
			// event on resource
			e := event.NewEvent(
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

	}
	return events
}
