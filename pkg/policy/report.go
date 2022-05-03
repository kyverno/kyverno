package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	changerequestlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	policyreportlister "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policyreport"
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

	pvInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)

	// as engineResponses holds the results for all matched resources in one namespace
	// we can merge pvInfos into a single object to reduce update frequency (throttling request) on RCR
	infos := mergePvInfos(pvInfos)
	for _, info := range infos {
		pc.prGenerator.Add(info)
		logger.V(4).Info("added a request to RCR generator", "key", info.ToKey())
	}
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *PolicyController) forceReconciliation(reconcileCh <-chan bool, stopCh <-chan struct{}) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)

	for {
		select {
		case <-ticker.C:
			logger.Info("performing the background scan", "scan interval", pc.reconcilePeriod.String())
			if err := pc.policyReportEraser.CleanupReportChangeRequests(cleanupReportChangeRequests); err != nil {
				logger.Error(err, "failed to cleanup report change requests")
			}

			if err := pc.policyReportEraser.EraseResultsEntries(eraseResultsEntries); err != nil {
				logger.Error(err, "continue reconciling policy reports")
			}

			pc.requeuePolicies()

		case erase := <-reconcileCh:
			logger.Info("received the reconcile signal, reconciling policy report")
			if err := pc.policyReportEraser.CleanupReportChangeRequests(cleanupReportChangeRequests); err != nil {
				logger.Error(err, "failed to cleanup report change requests")
			}

			if erase {
				if err := pc.policyReportEraser.EraseResultsEntries(eraseResultsEntries); err != nil {
					logger.Error(err, "continue reconciling policy reports")
				}
			}

			pc.requeuePolicies()

		case <-stopCh:
			return
		}
	}
}

func cleanupReportChangeRequests(pclient kyvernoclient.Interface, rcrLister changerequestlister.ReportChangeRequestLister, crcrLister changerequestlister.ClusterReportChangeRequestLister) error {
	var errors []string

	var gracePeriod int64 = 0
	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}

	err := pclient.KyvernoV1alpha2().ClusterReportChangeRequests().DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err.Error())
	}

	err = pclient.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace).DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{})
	if err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("%v", strings.Join(errors, ";"))
}

func eraseResultsEntries(pclient kyvernoclient.Interface, reportLister policyreportlister.PolicyReportLister, clusterReportLister policyreportlister.ClusterPolicyReportLister) error {
	selector, err := metav1.LabelSelectorAsSelector(policyreport.LabelSelector)
	if err != nil {
		return fmt.Errorf("failed to erase results entries %v", err)
	}

	var errors []string

	if polrs, err := reportLister.List(selector); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, polr := range polrs {
			polr.Results = []v1alpha2.PolicyReportResult{}
			polr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
				errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", polr.Kind, polr.Namespace, polr.Name, err))
			}
		}
	}

	if cpolrs, err := clusterReportLister.List(selector); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, cpolr := range cpolrs {
			cpolr.Results = []v1alpha2.PolicyReportResult{}
			cpolr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
				errors = append(errors, fmt.Sprintf("%s/%s: %v", cpolr.Kind, cpolr.Name, err))
			}
		}
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("failed to erase results entries %v", strings.Join(errors, ";"))
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
			eventInfos = append(eventInfos, *e)
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
		if rule.Status == response.RuleStatusPass {
			continue
		}

		eventResource := event.NewResourceViolationEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i])
		eventInfos = append(eventInfos, *eventResource)

		eventPolicy := event.NewPolicyFailEvent(event.PolicyController, event.PolicyViolation, er, &er.PolicyResponse.Rules[i], false)
		eventInfos = append(eventInfos, *eventPolicy)
	}

	if len(eventInfos) > 0 {
		logger.V(4).Info("generating events for policy", "events", eventInfos)
	}

	return eventInfos
}

func mergePvInfos(infos []policyreport.Info) []policyreport.Info {
	aggregatedInfo := []policyreport.Info{}
	if len(infos) == 0 {
		return nil
	}

	aggregatedInfoPerNamespace := make(map[string]policyreport.Info)
	for _, info := range infos {
		if tmpInfo, ok := aggregatedInfoPerNamespace[info.Namespace]; !ok {
			aggregatedInfoPerNamespace[info.Namespace] = info
		} else {
			tmpInfo.Results = append(tmpInfo.Results, info.Results...)
			aggregatedInfoPerNamespace[info.Namespace] = tmpInfo
		}

	}

	for _, i := range aggregatedInfoPerNamespace {
		aggregatedInfo = append(aggregatedInfo, i)
	}
	return aggregatedInfo
}
