package policyreport

import (
	"context"
	"sync"

	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	policyreportv1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type PolicyReportEraser interface {
	CleanupReportChangeRequests(ns string) error
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
func (e *eraser) CleanupReportChangeRequests(ns string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	var selector map[string]string
	if ns != "" {
		selector = map[string]string{ResourceLabelNamespace: ns}
	}
	return e.cleanupReportChangeRequests(selector)
}

func (e *eraser) EraseResultEntries(ns *string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return e.eraseResultEntries(ns)
}

func (e *eraser) cleanupReportChangeRequests(nslabels map[string]string) error {
	var errors []error
	var gracePeriod int64 = 0
	deleteOptions := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}

	selector := labels.SelectorFromSet(labels.Set(nslabels))

	err := e.pclient.KyvernoV1alpha2().ClusterReportChangeRequests().DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err)
	}

	err = e.pclient.KyvernoV1alpha2().ReportChangeRequests(config.KyvernoNamespace()).DeleteCollection(context.TODO(), deleteOptions, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		errors = append(errors, err)
	}

	if len(errors) == 0 {
		return nil
	}

	return multierr.Combine(errors...)
}

func (e *eraser) eraseResultEntries(ns *string) error {
	selector, err := metav1.LabelSelectorAsSelector(LabelSelector)
	if err != nil {
		return errors.Wrapf(err, "failed to erase results entries %v", err)
	}

	var errs []error
	var polrName string

	if ns != nil {
		if toggle.SplitPolicyReport.Enabled() {
			err = e.eraseSplitResultEntries(ns, selector)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			polrName = GeneratePolicyReportName(*ns, "")
			if polrName != "" {
				polr, err := e.reportLister.PolicyReports(*ns).Get(polrName)
				if err != nil {
					return errors.Wrapf(err, "failed to erase results entries for PolicyReport %s: %v", polrName, err)
				}

				polr.Results = []v1alpha2.PolicyReportResult{}
				polr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err = e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
					errs = append(errs, err)
				}
			} else {
				cpolr, err := e.clusterReportLister.Get(GeneratePolicyReportName(*ns, ""))
				if err != nil {
					errs = append(errs, err)
				}

				cpolr.Results = []v1alpha2.PolicyReportResult{}
				cpolr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err = e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
					return errors.Wrapf(err, "failed to erase results entries for ClusterPolicyReport %s: %v", polrName, err)
				}
			}
		}
		if len(errs) == 0 {
			return nil
		}

		return multierr.Combine(errs...)
	}

	if polrs, err := e.reportLister.List(selector); err != nil {
		errs = append(errs, err)
	} else {
		for _, polr := range polrs {
			polr.Results = []v1alpha2.PolicyReportResult{}
			polr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), polr, metav1.UpdateOptions{}); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if cpolrs, err := e.clusterReportLister.List(selector); err != nil {
		errs = append(errs, err)
	} else {
		for _, cpolr := range cpolrs {
			cpolr.Results = []v1alpha2.PolicyReportResult{}
			cpolr.Summary = v1alpha2.PolicyReportSummary{}
			if _, err = e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), cpolr, metav1.UpdateOptions{}); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return multierr.Combine(errs...)
}

func (e *eraser) eraseSplitResultEntries(ns *string, selector labels.Selector) error {
	var errs []error

	if ns != nil {
		if *ns != "" {
			polrs, err := e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(*ns).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
			if err != nil {
				return errors.Wrapf(err, "failed to list PolicyReports for given namespace %s : %v", *ns, err)
			}
			for _, polr := range polrs.Items {
				polr := polr
				polr.Results = []v1alpha2.PolicyReportResult{}
				polr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err := e.pclient.Wgpolicyk8sV1alpha2().PolicyReports(polr.GetNamespace()).Update(context.TODO(), &polr, metav1.UpdateOptions{}); err != nil {
					errs = append(errs, err)
				}
			}
		} else {
			cpolrs, err := e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
			if err != nil {
				return errors.Wrapf(err, "failed to list ClusterPolicyReports : %v", err)
			}
			for _, cpolr := range cpolrs.Items {
				cpolr := cpolr
				cpolr.Results = []v1alpha2.PolicyReportResult{}
				cpolr.Summary = v1alpha2.PolicyReportSummary{}
				if _, err := e.pclient.Wgpolicyk8sV1alpha2().ClusterPolicyReports().Update(context.TODO(), &cpolr, metav1.UpdateOptions{}); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if len(errs) == 0 {
			return nil
		}
	}
	return multierr.Combine(errs...)
}
