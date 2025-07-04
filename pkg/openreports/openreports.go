package openreports

import (
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
)

type ReportAdapter struct {
	*openreportsv1alpha1.Report
	Results []openreportsv1alpha1.ReportResult
}

type ClusterReportAdapter struct {
	*openreportsv1alpha1.ClusterReport
	Results []openreportsv1alpha1.ReportResult
}

func (a *ClusterReportAdapter) GetResults() []openreportsv1alpha1.ReportResult {
	return a.ClusterReport.Results
}

func (a *ClusterReportAdapter) SetResults(res []openreportsv1alpha1.ReportResult) {
	a.ClusterReport.Results = res
}

func (a *ClusterReportAdapter) SetSummary(s openreportsv1alpha1.ReportSummary) {
	a.ClusterReport.Summary = s
}

func (a *ReportAdapter) GetResults() []openreportsv1alpha1.ReportResult {
	return a.Report.Results
}

func (a *ReportAdapter) SetResults(res []openreportsv1alpha1.ReportResult) {
	a.Report.Results = res
}

func (a *ReportAdapter) SetSummary(s openreportsv1alpha1.ReportSummary) {
	a.Report.Summary = s
}
