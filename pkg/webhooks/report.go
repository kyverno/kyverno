package webhooks

import (
	"fmt"
	"strings"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/policyviolation"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/event"
)

//generateEvents generates event info for the engine responses
func generateEvents(engineResponses []engine.EngineResponse, onUpdate bool) []event.Info {
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
				e = event.NewEventNew(
					er.PolicyResponse.Resource.Kind,
					er.PolicyResponse.Resource.APIVersion,
					er.PolicyResponse.Resource.Namespace,
					er.PolicyResponse.Resource.Name,
					reason.String(),
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
					reason.String(),
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

func generatePV(ers []engine.EngineResponse, blocked bool) []policyviolation.Info {
	var pvInfos []policyviolation.Info
	// generate PV for each
	for _, er := range ers {
		// ignore creation of PV for resoruces that are yet to be assigned a name
		if er.IsSuccesful() {
			continue
		}
		glog.V(4).Infof("Building policy violation for engine response %v", er)
		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er, blocked))
	}
	return pvInfos
}

func buildPVInfo(er engine.EngineResponse, blocked bool) policyviolation.Info {
	info := policyviolation.Info{
		Blocked:    blocked,
		PolicyName: er.PolicyResponse.Policy,
		Resource:   er.PatchedResource,
		Rules:      buildViolatedRules(er, blocked),
	}
	return info
}

func buildViolatedRules(er engine.EngineResponse, blocked bool) []kyverno.ViolatedRule {
	blockMsg := fmt.Sprintf("Request Blocked for resource %s/%s; ", er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Kind)
	var violatedRules []kyverno.ViolatedRule
	// if resource was blocked we create dependent
	dependant := kyverno.ManagedResourceSpec{
		Kind:            er.PolicyResponse.Resource.Kind,
		Namespace:       er.PolicyResponse.Resource.Namespace,
		CreationBlocked: true,
	}

	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		vrule := kyverno.ViolatedRule{
			Name: rule.Name,
			Type: rule.Type,
		}

		if blocked {
			vrule.Message = blockMsg + rule.Message
			vrule.ManagedResource = dependant
		} else {
			vrule.Message = rule.Message
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}
