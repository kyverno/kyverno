package policyreport

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sync"
)

type PolicyReport struct {
	report        *policyreportv1alpha1.PolicyReport
	clusterReport *policyreportv1alpha1.ClusterPolicyReport
	violation     *kyverno.PolicyViolationTemplate
	mux           sync.Mutex
}

func NewPolicyReport(report *policyreportv1alpha1.PolicyReport, clusterReport *policyreportv1alpha1.ClusterPolicyReport, violation *kyverno.PolicyViolationTemplate) *PolicyReport {
	return &PolicyReport{
		report:        report,
		clusterReport: clusterReport,
		violation:     violation,
	}
}

// RemovePolicyViolation
func (p *PolicyReport) RemovePolicyViolation(name string, pvInfo []Info) *policyreportv1alpha1.PolicyReport {
	defer func() {
		p.mux.Unlock()
	}()
	p.mux.Lock()
	if len(pvInfo) > 0 {
		for _, info := range pvInfo {
			for _, v := range pvInfo[0].Rules {
				for i, result := range p.report.Results {
					if result.Resource.Name == name && result.Policy == info.PolicyName && result.Rule == v.Name {
						p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
					}
				}
			}
		}
	} else {
		for _, result := range p.report.Results {
			if result.Resource.Name == name {
				result.Message = ""
				result.Data["status"] = "Pass"
				result.Status = "Pass"
				p.DecreaseCountReport(p.report, string(result.Status))
				p.IncreaseCountReport(p.report, string(result.Status))
			}
		}
	}
	return p.report
}

//RemoveClusterPolicyViolation
func (p *PolicyReport) RemoveClusterPolicyViolation(name string, pvInfo []Info) *policyreportv1alpha1.ClusterPolicyReport {
	defer func() {
		p.mux.Unlock()
	}()
	p.mux.Lock()
	if len(pvInfo) > 0 {
		for _, info := range pvInfo {
			for _, v := range info.Rules {
				for i, result := range p.clusterReport.Results {
					if result.Resource.Name == name && result.Policy == info.PolicyName && result.Rule == v.Name {
						if v.Check != result.Data["status"] {
							p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
						}
					}
				}
			}
		}
	} else {
		for _, result := range p.clusterReport.Results {
			if result.Resource.Name == name {
				result.Message = ""
				result.Status = "Pass"
				p.DecreaseCountClusterReport(p.clusterReport, string(result.Status))
				p.IncreaseCountClusterReport(p.clusterReport, string(result.Status))
			}
		}
	}
	return p.clusterReport
}

// CreatePolicyViolationToPolicyReport
func (p *PolicyReport) CreatePolicyViolationToPolicyReport() *policyreportv1alpha1.PolicyReport {
	defer func() {
		p.mux.Unlock()
	}()
	p.mux.Lock()
	for _, result := range p.report.Results {
		for i, rule := range p.violation.Spec.ViolatedRules {
			if result.Policy == p.violation.Spec.Policy && result.Rule == rule.Name && result.Resource.Name == p.violation.Spec.Name {
				result.Message = rule.Message
				p.DecreaseCountReport(p.report, string(result.Status))
				p.IncreaseCountReport(p.report, rule.Check)
				p.violation.Spec.ViolatedRules = append(p.violation.Spec.ViolatedRules[:i], p.violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _, rule := range p.violation.Spec.ViolatedRules {
		result := &policyreportv1alpha1.PolicyReportResult{
			Policy:  p.violation.Spec.Policy,
			Rule:    rule.Name,
			Message: rule.Message,
			Status:  policyreportv1alpha1.PolicyStatus(rule.Check),
			Resource: &corev1.ObjectReference{
				Kind:       p.violation.Spec.Kind,
				Namespace:  p.violation.Spec.Namespace,
				APIVersion: p.violation.Spec.APIVersion,
				Name:       p.violation.Spec.Name,
			},
		}
		p.IncreaseCountReport(p.report, rule.Check)
		p.report.Results = append(p.report.Results, result)
	}
	return p.report
}

// ClusterPolicyViolationsToClusterPolicyReport
func (p *PolicyReport) CreateClusterPolicyViolationsToClusterPolicyReport() *policyreportv1alpha1.ClusterPolicyReport {
	defer func() {
		p.mux.Unlock()
	}()
	p.mux.Lock()
	for _, result := range p.clusterReport.Results {
		for i, rule := range p.violation.Spec.ViolatedRules {
			if result.Policy == p.violation.Spec.Policy && result.Rule == rule.Name && result.Resource.Name == p.violation.Spec.Name {
				result.Message = rule.Message
				p.DecreaseCountClusterReport(p.clusterReport, string(result.Status))
				p.IncreaseCountClusterReport(p.clusterReport, rule.Check)
				p.violation.Spec.ViolatedRules = append(p.violation.Spec.ViolatedRules[:i], p.violation.Spec.ViolatedRules[i+1:]...)
			}
		}
	}
	for _, rule := range p.violation.Spec.ViolatedRules {
		result := &policyreportv1alpha1.PolicyReportResult{
			Policy:  p.violation.Spec.Policy,
			Rule:    rule.Name,
			Message: rule.Message,
			Status:  policyreportv1alpha1.PolicyStatus(rule.Check),
			Resource: &corev1.ObjectReference{
				Kind:       p.violation.Spec.Kind,
				Namespace:  p.violation.Spec.Namespace,
				APIVersion: p.violation.Spec.APIVersion,
				Name:       p.violation.Spec.Name,
			},
		}
		p.IncreaseCountClusterReport(p.clusterReport, rule.Check)
		p.clusterReport.Results = append(p.clusterReport.Results, result)
	}
	return p.clusterReport
}

func (p *PolicyReport) DecreaseCountClusterReport(reports *policyreportv1alpha1.ClusterPolicyReport, status string) {
	switch status {
	case "Pass":
		reports.Summary.Pass--
		break
	case "Fail":
		reports.Summary.Fail--
		break
	default:
		reports.Summary.Skip--
		break
	}
}

func (p *PolicyReport) IncreaseCountClusterReport(reports *policyreportv1alpha1.ClusterPolicyReport, status string) {
	switch status {
	case "Pass":
		reports.Summary.Pass++
		break
	case "Fail":
		reports.Summary.Fail++
		break
	default:
		reports.Summary.Skip++
		break
	}
}

func (p *PolicyReport) IncreaseCountReport(reports *policyreportv1alpha1.PolicyReport, status string) {
	switch status {
	case "Pass":
		reports.Summary.Pass++
		break
	case "Fail":
		reports.Summary.Fail++
		break
	default:
		reports.Summary.Skip++
		break
	}
}

func (p *PolicyReport) DecreaseCountReport(reports *policyreportv1alpha1.PolicyReport, status string) {
	switch status {
	case "Pass":
		reports.Summary.Pass--
		break
	case "Fail":
		reports.Summary.Fail--
		break
	default:
		reports.Summary.Skip--
		break
	}
}
