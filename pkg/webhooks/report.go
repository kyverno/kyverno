package webhooks

import (
	"strings"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []engine.EngineResponseNew, onUpdate bool) []event.Info {
	var events []event.Info
	if !isResponseSuccesful(engineResponses) {
		for _, er := range engineResponses {
			if er.IsSuccesful() {
				// dont create events on success
				continue
			}
			failedRules := er.GetFailedRules()
			filedRulesStr := strings.Join(failedRules, ";")
			if onUpdate {
				var e event.Info
				// UPDATE
				// event on resource
				e = event.NewEventNew(
					er.PolicyResponse.Resource.Kind,
					er.PolicyResponse.Resource.APIVersion,
					er.PolicyResponse.Resource.Namespace,
					er.PolicyResponse.Resource.Name,
					event.RequestBlocked.String(),
					event.FPolicyApplyBlockUpdate,
					filedRulesStr,
					er.PolicyResponse.Policy,
				)
				glog.V(4).Infof("UPDATE event on resource %s/%s/%s with policy %s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name, er.PolicyResponse.Policy)
				events = append(events, e)

				// event on policy
				e = event.NewEventNew(
					"ClusterPolicy",
					kyverno.SchemeGroupVersion.String(),
					"",
					er.PolicyResponse.Policy,
					event.RequestBlocked.String(),
					event.FPolicyBlockResourceUpdate,
					er.PolicyResponse.Resource.Namespace+"/"+er.PolicyResponse.Resource.Name,
					filedRulesStr,
				)
				glog.V(4).Infof("UPDATE event on policy %s", er.PolicyResponse.Policy)
				events = append(events, e)

			} else {
				// CREATE
				// event on policy
				e := event.NewEventNew(
					"ClusterPolicy",
					kyverno.SchemeGroupVersion.String(),
					"",
					er.PolicyResponse.Policy,
					event.RequestBlocked.String(),
					event.FPolicyApplyBlockCreate,
					er.PolicyResponse.Resource.Namespace+"/"+er.PolicyResponse.Resource.Name,
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
			e := event.NewEventNew(
				er.PolicyResponse.Resource.Kind,
				er.PolicyResponse.Resource.APIVersion,
				er.PolicyResponse.Resource.Namespace,
				er.PolicyResponse.Resource.Name,
				event.PolicyApplied.String(),
				event.SRulesApply,
				successRulesStr,
				er.PolicyResponse.Policy,
			)
			events = append(events, e)
		}

	}
	return events
}
