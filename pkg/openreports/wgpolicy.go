package openreports

import (
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

type WgpolicyReportAdapter struct {
	*policyreportv1alpha2.PolicyReport
	or *ReportAdapter
}

type WgpolicyClusterReportAdapter struct {
	*policyreportv1alpha2.ClusterPolicyReport
	or *ClusterReportAdapter
}

func (a *WgpolicyClusterReportAdapter) GetResults() []openreportsv1alpha1.ReportResult {
	return a.or.GetResults()
}

func (a *WgpolicyClusterReportAdapter) SetResults(res []openreportsv1alpha1.ReportResult) {
	wgpolResults := []policyreportv1alpha2.PolicyReportResult{}
	for _, r := range res {
		wgRes := policyreportv1alpha2.PolicyReportResult{
			Source:           r.Source,
			Policy:           r.Policy,
			Resources:        r.Subjects,
			Rule:             r.Rule,
			Message:          r.Description,
			Severity:         policyreportv1alpha2.PolicySeverity(r.Severity),
			Result:           policyreportv1alpha2.PolicyResult(r.Result),
			ResourceSelector: r.ResourceSelector,
			Scored:           r.Scored,
			Timestamp:        r.Timestamp,
			Properties:       r.Properties,
			Category:         r.Category,
		}
		wgpolResults = append(wgpolResults, wgRes)
	}
	a.ClusterPolicyReport.Results = wgpolResults
	a.or = &ClusterReportAdapter{ClusterReport: a.ClusterPolicyReport.ToOpenReports()}
}

func (a *WgpolicyClusterReportAdapter) SetSummary(s openreportsv1alpha1.ReportSummary) {
	a.ClusterPolicyReport.Summary = policyreportv1alpha2.PolicyReportSummary{
		Pass:  s.Pass,
		Fail:  s.Fail,
		Warn:  s.Warn,
		Skip:  s.Skip,
		Error: s.Error,
	}
	a.or = &ClusterReportAdapter{ClusterReport: a.ClusterPolicyReport.ToOpenReports()}
}

func (a *WgpolicyReportAdapter) GetResults() []openreportsv1alpha1.ReportResult {
	return a.or.GetResults()
}

func (a *WgpolicyReportAdapter) SetResults(res []openreportsv1alpha1.ReportResult) {
	wgpolResults := []policyreportv1alpha2.PolicyReportResult{}
	for _, r := range res {
		wgRes := policyreportv1alpha2.PolicyReportResult{
			Source:           r.Source,
			Policy:           r.Policy,
			Resources:        r.Subjects,
			Rule:             r.Rule,
			Message:          r.Description,
			Severity:         policyreportv1alpha2.PolicySeverity(r.Severity),
			Result:           policyreportv1alpha2.PolicyResult(r.Result),
			ResourceSelector: r.ResourceSelector,
			Scored:           r.Scored,
			Timestamp:        r.Timestamp,
			Properties:       r.Properties,
			Category:         r.Category,
		}
		wgpolResults = append(wgpolResults, wgRes)
	}
	a.PolicyReport.Results = wgpolResults
	a.or = &ReportAdapter{Report: a.PolicyReport.ToOpenReports()}
}

func (a *WgpolicyReportAdapter) SetSummary(s openreportsv1alpha1.ReportSummary) {
	a.PolicyReport.Summary = policyreportv1alpha2.PolicyReportSummary{
		Pass:  s.Pass,
		Fail:  s.Fail,
		Warn:  s.Warn,
		Skip:  s.Skip,
		Error: s.Error,
	}
	a.or = &ReportAdapter{Report: a.PolicyReport.ToOpenReports()}
}

func NewWGPolAdapter(polr *policyreportv1alpha2.PolicyReport) *WgpolicyReportAdapter {
	return &WgpolicyReportAdapter{
		PolicyReport: polr,
		or:           &ReportAdapter{Report: polr.ToOpenReports()},
	}
}

func NewWGCpolAdapter(polr *policyreportv1alpha2.ClusterPolicyReport) *WgpolicyClusterReportAdapter {
	return &WgpolicyClusterReportAdapter{
		ClusterPolicyReport: polr,
		or:                  &ClusterReportAdapter{ClusterReport: polr.ToOpenReports()},
	}
}
