package policy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1alpha1 "github.com/kyverno/kyverno/pkg/api/policyreport/v1alpha1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policyreportlister "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/policyreport"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) report(policy string, engineResponses []*response.EngineResponse, logger logr.Logger) {
	eventInfos := generateEvents(logger, engineResponses)
	pc.eventGen.Add(eventInfos...)

	pvInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)

	// as engineResponses holds the results for all matched resources in one namespace
	// we can merge pvInfos into a single object to reduce update frequency (throttling request) on RCR
	info := mergePvInfos(pvInfos)
	pc.prGenerator.Add(info)
	logger.V(4).Info("added a request to RCR generator", "key", info.ToKey())
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *PolicyController) forceReconciliation(reconcileCh <-chan bool, stopCh <-chan struct{}) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)

	for {
		select {
		case <-ticker.C:
			logger.Info("performing the background scan", "scan interval", pc.reconcilePeriod.String())
			if err := pc.policyReportEraser.EraseResultsEntries(eraseResultsEntries); err != nil {
				logger.Error(err, "continue reconciling policy reports")
			}

			pc.requeuePolicies()

		case erase := <-reconcileCh:
			logger.Info("received the reconcile signal, reconciling policy report")
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

func eraseResultsEntries(pclient *kyvernoclient.Clientset, reportLister policyreportlister.PolicyReportLister, clusterReportLister policyreportlister.ClusterPolicyReportLister) error {
	var errors []string

	if polrs, err := reportLister.List(labels.Everything()); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, polr := range polrs {
			polr.Results = []*v1alpha1.PolicyReportResult{}
			polr.Summary = v1alpha1.PolicyReportSummary{}
			if _, err = pclient.Wgpolicyk8sV1alpha1().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
				errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", polr.Kind, polr.Namespace, polr.Name, err))
			}
		}
	}

	if cpolrs, err := clusterReportLister.List(labels.Everything()); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, cpolr := range cpolrs {
			cpolr.Results = []*v1alpha1.PolicyReportResult{}
			cpolr.Summary = v1alpha1.PolicyReportSummary{}
			if _, err = pclient.Wgpolicyk8sV1alpha1().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
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

	namespaces, err := pc.nsLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "unable to list namespaces")
		return
	}

	for _, ns := range namespaces {
		pols, err := pc.npLister.Policies(ns.GetName()).List(labels.Everything())
		if err != nil {
			logger.Error(err, "unable to list Policies", "namespace", ns.GetName())
			continue
		}

		for _, p := range pols {
			pol := ConvertPolicyToClusterPolicy(p)
			if !pc.canBackgroundProcess(pol) {
				continue
			}
			pc.enqueuePolicy(pol)
		}
	}
}

func generateEvents(log logr.Logger, ers []*response.EngineResponse) []event.Info {
	var eventInfos []event.Info
	for _, er := range ers {
		if er.IsSuccessful() {
			continue
		}
		eventInfos = append(eventInfos, generateEventsPerEr(log, er)...)
	}
	return eventInfos
}

func generateEventsPerEr(log logr.Logger, er *response.EngineResponse) []event.Info {
	var eventInfos []event.Info

	logger := log.WithValues("policy", er.PolicyResponse.Policy, "kind", er.PolicyResponse.Resource.Kind, "namespace", er.PolicyResponse.Resource.Namespace, "name", er.PolicyResponse.Resource.Name)
	logger.V(4).Info("reporting results for policy")

	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		// generate event on resource for each failed rule
		logger.V(4).Info("generating event on resource")
		e := event.Info{}
		e.Kind = er.PolicyResponse.Resource.Kind
		e.Namespace = er.PolicyResponse.Resource.Namespace
		e.Name = er.PolicyResponse.Resource.Name
		e.Reason = event.PolicyViolation.String()
		e.Source = event.PolicyController
		e.Message = fmt.Sprintf("policy '%s' (%s) rule '%s' failed. %v", er.PolicyResponse.Policy, rule.Type, rule.Name, rule.Message)
		eventInfos = append(eventInfos, e)
	}

	return eventInfos
}

func mergePvInfos(infos []policyreport.Info) policyreport.Info {
	aggregatedInfo := policyreport.Info{}
	if len(infos) == 0 {
		return aggregatedInfo
	}

	var results []policyreport.EngineResponseResult
	for _, info := range infos {
		for _, res := range info.Results {
			results = append(results, res)
		}
	}

	aggregatedInfo.PolicyName = infos[0].PolicyName
	aggregatedInfo.Namespace = infos[0].Namespace
	aggregatedInfo.Results = results
	return aggregatedInfo
}
