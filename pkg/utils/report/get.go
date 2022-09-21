package report

import (
	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
)

func GetAdmissionReport(
	namespace string,
	name string,
	lister kyvernov1alpha2listers.AdmissionReportLister,
	cLister kyvernov1alpha2listers.ClusterAdmissionReportLister,
) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	if namespace == "" {
		report, err := cLister.Get(name)
		if err != nil {
			return nil, err
		}
		return report.DeepCopy(), nil
	} else {
		report, err := lister.AdmissionReports(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return report.DeepCopy(), nil
	}
}

func GetBackgroungScanReport(
	namespace string,
	name string,
	lister kyvernov1alpha2listers.BackgroundScanReportLister,
	cLister kyvernov1alpha2listers.ClusterBackgroundScanReportLister,
) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	if namespace == "" {
		report, err := cLister.Get(name)
		if err != nil {
			return nil, err
		}
		return report.DeepCopy(), nil
	} else {
		report, err := lister.BackgroundScanReports(namespace).Get(name)
		if err != nil {
			return nil, err
		}
		return report.DeepCopy(), nil
	}
}
