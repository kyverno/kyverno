package policyreport

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sync"
)

type PolicyReport struct {
	report        *policyreportv1alpha1.PolicyReport
	clusterReport *policyreportv1alpha1.ClusterPolicyReport
	violation     *kyverno.PolicyViolationTemplate
	k8sClient *client.Client
	mux           sync.Mutex
}

func NewPolicyReport(report *policyreportv1alpha1.PolicyReport, clusterReport *policyreportv1alpha1.ClusterPolicyReport, violation *kyverno.PolicyViolationTemplate,client *client.Client) *PolicyReport {
	return &PolicyReport{
		report:        report,
		clusterReport: clusterReport,
		violation:     violation,
		k8sClient : client,
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
						_, err := p.k8sClient.GetResource(result.Resource.APIVersion,result.Resource.Kind,result.Resource.Namespace,result.Resource.Name,)
						if err != nil {
							if !errors.IsNotFound(err) {
								p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
							}
						}
						result.Message = v.Message
						p.DecreaseCountReport(p.report, string(result.Status))
						result.Status = policyreportv1alpha1.PolicyStatus(v.Check)
						p.IncreaseCountReport(p.report, string(v.Check))
					}
				}
			}
		}
	} else {
		for i, result := range p.report.Results {
			if result.Policy == name {
				p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
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
						_, err := p.k8sClient.GetResource(result.Resource.APIVersion,result.Resource.Kind,result.Resource.Namespace,result.Resource.Name,)
						if err != nil {
							if !errors.IsNotFound(err) {
								p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
							}
						}
						result.Message = v.Message
						p.DecreaseCountReport(p.report, string(result.Status))
						result.Status = policyreportv1alpha1.PolicyStatus(v.Check)
						p.IncreaseCountReport(p.report, string(v.Check))
					}
				}
			}
		}
	} else {
		for i, result := range p.clusterReport.Results {
			if result.Policy == name {
				p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
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
