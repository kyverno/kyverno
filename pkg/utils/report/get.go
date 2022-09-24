package report

import (
	"context"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	wgpolicyk8sv1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	kyvernov1alpha2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAdmissionReport(
	namespace string,
	name string,
	lister kyvernov1alpha2listers.AdmissionReportLister,
	cLister kyvernov1alpha2listers.ClusterAdmissionReportLister,
) (kyvernov1alpha2.ReportInterface, error) {
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
) (kyvernov1alpha2.ReportInterface, error) {
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

func GetPolicyReports(
	namespace string,
	// lister policyreportv1alpha2listers.PolicyReportLister,
	// cLister policyreportv1alpha2listers.ClusterPolicyReportLister,
	client wgpolicyk8sv1alpha2.Wgpolicyk8sV1alpha2Interface,
) ([]kyvernov1alpha2.ReportInterface, error) {
	var reports []kyvernov1alpha2.ReportInterface
	if namespace == "" {
		list, err := client.ClusterPolicyReports().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			reports = append(reports, &list.Items[i])
		}
	} else {
		list, err := client.PolicyReports(namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			reports = append(reports, &list.Items[i])
		}
	}
	return reports, nil
}
