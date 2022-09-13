package policyreport

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type PolicyReportEraser interface {
	CleanupReportChangeRequests(labels map[string]string) error
	EraseResultEntries(ns *string) error
}

func NewPolicyReportEraser(
	pclient versioned.Interface,
	reportLister policyreportv1alpha2listers.PolicyReportLister,
	clusterReportLister policyreportv1alpha2listers.ClusterPolicyReportLister,
) PolicyReportEraser {
	return &eraser{
		pclient:             pclient,
		reportLister:        reportLister,
		clusterReportLister: clusterReportLister,
		mutex:               &sync.RWMutex{},
	}
}

type eraser struct {
	pclient             versioned.Interface
	reportLister        policyreportv1alpha2listers.PolicyReportLister
	clusterReportLister policyreportv1alpha2listers.ClusterPolicyReportLister
	mutex               *sync.RWMutex
}

// TODO: make sure both functions below can be synced separately
func (e *eraser) CleanupReportChangeRequests(labels map[string]string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.cleanupReportChangeRequests(labels)
}

func (e *eraser) EraseResultEntries(ns *string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.eraseResultEntries(ns)
}

func (e *eraser) cleanupReportChangeRequests(nslabels map[string]string) error {
	var errors []string
	var gracePeriod int64 = 0
	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}

	selector := labels.SelectorFromSet(labels.Set(nslabels))

	err := e.pclient.KyvernoV1alpha2().ClusterReportChangeRequests().DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err.Error())
	}

	err = e.pclient.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace()).DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err.Error())
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("%v", strings.Join(errors, ";"))
}

func (e *eraser) eraseResultEntries(ns *string) error {
	selector, err := metav1.LabelSelectorAsSelector(LabelSelector)
	if err != nil {
		return fmt.Errorf("failed to erase results entries %v", err)
	}

	var errors []string
	var polrName string

	if ns != nil {
		if toggle.SplitPolicyReport.Enabled() {
			err = e.eraseSplitResultEntries(ns, selector)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%v", err))
			}
		} else {
			polrName = GeneratePolicyReportName(*ns, "")
			if polrName != "" {
				polr, err := e.reportLister.PolicyReports(*ns).Get(polrName)
				if err != nil {
					return fmt.Errorf("failed to erase results entries for PolicyReport %s: %v", polrName, err)
				}

				polr.Results = []v1alpha2.PolicyReportResult{}
				polr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err = e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
					errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", polr.Kind, polr.Namespace, polr.Name, err))
				}
			} else {
				cpolr, err := e.clusterReportLister.Get(GeneratePolicyReportName(*ns, ""))
				if err != nil {
					errors = append(errors, err.Error())
				}

				cpolr.Results = []v1alpha2.PolicyReportResult{}
				cpolr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err = e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
					return fmt.Errorf("failed to erase results entries for ClusterPolicyReport %s: %v", polrName, err)
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}

		return fmt.Errorf("failed to erase results entries for report %s: %v", polrName, strings.Join(errors, ";"))
	}

	if polrs, err := e.reportLister.List(selector); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, polr := range polrs {
			polr.Results = []v1alpha2.PolicyReportResult{}
			polr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
				errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", polr.Kind, polr.Namespace, polr.Name, err))
			}
		}
	}

	if cpolrs, err := e.clusterReportLister.List(selector); err != nil {
		errors = append(errors, err.Error())
	} else {
		for _, cpolr := range cpolrs {
			cpolr.Results = []v1alpha2.PolicyReportResult{}
			cpolr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
				errors = append(errors, fmt.Sprintf("%s/%s: %v", cpolr.Kind, cpolr.Name, err))
			}
		}
	}

	if len(errors) == 0 {
		return nil
	}

	return fmt.Errorf("failed to erase results entries %v", strings.Join(errors, ";"))
}

func (e *eraser) eraseSplitResultEntries(ns *string, selector labels.Selector) error {
	var errors []string

	if ns != nil {
		if *ns != "" {
			polrs, err := e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(*ns).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
			if err != nil {
				return fmt.Errorf("failed to list PolicyReports for given namespace %s : %v", *ns, err)
			}
			for _, polr := range polrs.Items {
				polr := polr
				polr.Results = []v1alpha2.PolicyReportResult{}
				polr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err := e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), &polr, metav1.UpdateOptions{}); err != nil {
					errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", polr.Kind, polr.Namespace, polr.Name, err))
				}
			}
		} else {
			cpolrs, err := e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
			if err != nil {
				return fmt.Errorf("failed to list ClusterPolicyReports : %v", err)
			}
			for _, cpolr := range cpolrs.Items {
				cpolr := cpolr
				cpolr.Results = []v1alpha2.PolicyReportResult{}
				cpolr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err := e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), &cpolr, metav1.UpdateOptions{}); err != nil {
					errors = append(errors, fmt.Sprintf("%s/%s/%s: %v", cpolr.Kind, cpolr.Namespace, cpolr.Name, err))
				}
			}
		}
		if len(errors) == 0 {
			return nil
		}
	}
	return fmt.Errorf("failed to erase results entries for split reports in namespace %s: %v", *ns, strings.Join(errors, ";"))
}
