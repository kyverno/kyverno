package policy

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) report(engineResponses []*response.EngineResponse, logger logr.Logger) {
	eventInfos := generateFailEvents(logger, engineResponses)
	pc.eventGen.Add(eventInfos...)

	if pc.configHandler.GetGenerateSuccessEvents() {
		successEventInfos := generateSuccessEvents(logger, engineResponses)
		pc.eventGen.Add(successEventInfos...)
	}
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *PolicyController) forceReconciliation(ctx context.Context) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)

	for {
		select {
		case <-ticker.C:
			logger.Info("performing the background scan", "scan interval", pc.reconcilePeriod.String())
			pc.requeuePolicies()

		case <-ctx.Done():
			return
		}
	}
}

func (pc *PolicyController) requeuePolicies() {
	logger := pc.log.WithName("requeuePolicies")
	if cpols, err := pc.pLister.List(labels.Everything()); err == nil {
		for _, cpol := range cpols {
			if !pc.canBackgroundProcess(cpol) {
				continue
			}
			pc.enqueuePolicy(cpol)
		}
	} else {
		logger.Error(err, "unable to list ClusterPolicies")
	}
	if pols, err := pc.npLister.Policies(metav1.NamespaceAll).List(labels.Everything()); err == nil {
		for _, p := range pols {
			if !pc.canBackgroundProcess(p) {
				continue
			}
			pc.enqueuePolicy(p)
		}
	} else {
		logger.Error(err, "unable to list Policies")
	}
}

func generateSuccessEvents(log logr.Logger, ers []*response.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		logger := log.WithValues("policy", er.PolicyResponse.Policy, "kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace, "name", er.PolicyResponse.Resource.Name)
		if !er.IsFailed() {
			logger.V(4).Info("generating event on policy for success rules")
			e := event.NewPolicyAppliedEvent(event.PolicyController, er)
			eventInfos = append(eventInfos, e)
		}
	}

	return eventInfos
}

func generateFailEvents(log logr.Logger, ers []*response.EngineResponse) (eventInfos []event.Info) {
	for _, er := range ers {
		eventInfos = append(eventInfos, generateFailEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateFailEventsPerEr(log logr.Logger, er *response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	logger := log.WithValues("policy", er.PolicyResponse.Policy.Name,
		"kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace,
		"name", er.PolicyResponse.Resource.Name)

	for i, rule := range er.PolicyResponse.Rules {
		if rule.Status != response.RuleStatusPass && rule.Status != response.RuleStatusSkip {
			eventResource := event.NewResourceViolationEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i])
			eventInfos = append(eventInfos, eventResource)

			eventPolicy := event.NewPolicyFailEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i], false)
			eventInfos = append(eventInfos, eventPolicy)
		}
	}

	if len(eventInfos) > 0 {
		logger.V(4).Info("generating events for policy", "events", eventInfos)
	}

	return eventInfos
}
