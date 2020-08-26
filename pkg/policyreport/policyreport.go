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
	k8sClient     *client.Client
	mux           sync.Mutex
}

func NewPolicyReport(report *policyreportv1alpha1.PolicyReport, clusterReport *policyreportv1alpha1.ClusterPolicyReport, violation *kyverno.PolicyViolationTemplate, client *client.Client) *PolicyReport {
	return &PolicyReport{
		report:        report,
		clusterReport: clusterReport,
		violation:     violation,
		k8sClient:     client,
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
			for j, v := range pvInfo[0].Rules {
				for i, result := range p.report.Results {
					if result.Resource.Name == info.Resource.GetName() && result.Policy == info.PolicyName && result.Rule == v.Name {
						_, err := p.k8sClient.GetResource(result.Resource.APIVersion, result.Resource.Kind, result.Resource.Namespace, result.Resource.Name)
						if err != nil {
							if errors.IsNotFound(err) {
								p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
								p.DecreaseCount(string(result.Status), "NAMESPACE")
							}
						} else {
							if v.Check != string(result.Status) {
								result.Message = v.Message
								p.DecreaseCount(string(result.Status), "NAMESPACE")
								result.Status = policyreportv1alpha1.PolicyStatus(v.Check)
								p.IncreaseCount(string(v.Check), "NAMESPACE")
							}

							info.Rules = append(info.Rules[:j], info.Rules[j+1:]...)
						}

					}
				}
			}
		}
	} else {
		for i, result := range p.report.Results {
			if result.Policy == name {
				p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
				p.DecreaseCount(string(result.Status), "NAMESPACE")
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
			for j, v := range info.Rules {
				for i, result := range p.clusterReport.Results {
					if result.Resource.Name == info.Resource.GetName() && result.Policy == info.PolicyName && result.Rule == v.Name {
						_, err := p.k8sClient.GetResource(result.Resource.APIVersion, result.Resource.Kind, result.Resource.Namespace, result.Resource.Name)
						if err != nil {
							if errors.IsNotFound(err) {
								p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
								p.DecreaseCount(string(result.Status), "CLUSTER")
							}
						} else {
							if v.Check != string(result.Status) {
								result.Message = v.Message
								p.DecreaseCount(string(result.Status), "CLUSTER")
								result.Status = policyreportv1alpha1.PolicyStatus(v.Check)
								p.IncreaseCount(string(v.Check), "CLUSTER")
							}
							info.Rules = append(info.Rules[:j], info.Rules[j+1:]...)
						}

					}
				}
			}
		}
	} else {
		for i, result := range p.clusterReport.Results {
			if result.Policy == name {
				p.clusterReport.Results = append(p.clusterReport.Results[:i], p.clusterReport.Results[i+1:]...)
				p.DecreaseCount(string(result.Status), "CLUSTER")
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
				_, err := p.k8sClient.GetResource(result.Resource.APIVersion, result.Resource.Kind, result.Resource.Namespace, result.Resource.Name)
				if err != nil {
					if errors.IsNotFound(err) {
						p.report.Results = append(p.report.Results[:i], p.report.Results[i+1:]...)
					}
				} else {
					if rule.Check != string(result.Status) {
						p.DecreaseCount(string(result.Status), "NAMESPACE")
						p.IncreaseCount(rule.Check, "NAMESPACE")
						result.Message = rule.Message
					}
					p.violation.Spec.ViolatedRules = append(p.violation.Spec.ViolatedRules[:i], p.violation.Spec.ViolatedRules[i+1:]...)
				}

			}
		}
	}
	for _, rule := range p.violation.Spec.ViolatedRules {
		_, err := p.k8sClient.GetResource(p.violation.Spec.APIVersion, p.violation.Spec.Kind, p.violation.Spec.Namespace, p.violation.Spec.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
		} else {
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
			p.IncreaseCount(rule.Check, "NAMESPACE")
			p.report.Results = append(p.report.Results, result)
		}

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
				_, err := p.k8sClient.GetResource(result.Resource.APIVersion, result.Resource.Kind, result.Resource.Namespace, result.Resource.Name)
				if err != nil {
					if errors.IsNotFound(err) {
						p.clusterReport.Results = append(p.clusterReport.Results[:i], p.clusterReport.Results[i+1:]...)
						continue
					}
				} else {
					if rule.Check != string(result.Status) {
						result.Message = rule.Message
						p.DecreaseCount(string(result.Status), "CLUSTER")
						p.IncreaseCount(rule.Check, "CLUSTER")
					}
					p.violation.Spec.ViolatedRules = append(p.violation.Spec.ViolatedRules[:i], p.violation.Spec.ViolatedRules[i+1:]...)
				}
			}
		}
	}
	for _, rule := range p.violation.Spec.ViolatedRules {
		_, err := p.k8sClient.GetResource(p.violation.Spec.APIVersion, p.violation.Spec.Kind, p.violation.Spec.Namespace, p.violation.Spec.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
		} else {
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
			p.IncreaseCount(rule.Check, "CLUSTER")
			p.clusterReport.Results = append(p.clusterReport.Results, result)
		}

	}
	return p.clusterReport
}

func (p *PolicyReport) DecreaseCount(status string, scope string) {
	if scope == "CLUSTER" {
		switch status {
		case "Pass":
			if p.clusterReport.Summary.Pass--; p.clusterReport.Summary.Pass < 0 {
				p.clusterReport.Summary.Pass = 0
			}
			break
		case "Fail":
			if p.clusterReport.Summary.Fail--; p.clusterReport.Summary.Fail < 0 {
				p.clusterReport.Summary.Fail = 0
			}
			break
		default:
			if p.clusterReport.Summary.Skip--; p.clusterReport.Summary.Skip < 0 {
				p.clusterReport.Summary.Skip = 0
			}
			break
		}
	} else {
		switch status {
		case "Pass":
			if p.report.Summary.Pass--; p.report.Summary.Pass < 0 {
				p.report.Summary.Pass = 0
			}
			break
		case "Fail":
			if p.report.Summary.Fail--; p.report.Summary.Fail < 0 {
				p.report.Summary.Fail = 0
			}
			break
		default:
			if p.report.Summary.Skip--; p.report.Summary.Skip < 0 {
				p.report.Summary.Skip = 0
			}
			break
		}
	}

}

func (p *PolicyReport) IncreaseCount(status string, scope string) {
	if scope == "CLUSTER" {
		switch status {
		case "Pass":
			p.clusterReport.Summary.Pass++
			break
		case "Fail":
			p.clusterReport.Summary.Fail++
			break
		default:
			p.clusterReport.Summary.Skip++
			break
		}
	} else {
		switch status {
		case "Pass":
			p.report.Summary.Pass++
			break
		case "Fail":
			p.report.Summary.Fail++
			break
		default:
			p.report.Summary.Skip++
			break
		}
	}

}
