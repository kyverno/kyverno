package report

import (
	"context"
	"errors"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateReport(report kyvernov1alpha2.ReportChangeRequestInterface, client versioned.Interface) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	switch v := report.(type) {
	case *kyvernov1alpha2.AdmissionReport:
		report, err := client.KyvernoV1alpha2().AdmissionReports(report.GetNamespace()).Create(context.TODO(), v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterAdmissionReport:
		report, err := client.KyvernoV1alpha2().ClusterAdmissionReports().Create(context.TODO(), v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.ReportChangeRequest:
		report, err := client.KyvernoV1alpha2().ReportChangeRequests(report.GetNamespace()).Create(context.TODO(), v, metav1.CreateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterReportChangeRequest:
		report, err := client.KyvernoV1alpha2().ClusterReportChangeRequests().Create(context.TODO(), v, metav1.CreateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
