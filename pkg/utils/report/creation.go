package report

import (
	"context"
	"errors"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewReport(namespace, name string) kyvernov1alpha2.ReportChangeRequestInterface {
	var report kyvernov1alpha2.ReportChangeRequestInterface
	if namespace == "" {
		report = &kyvernov1alpha2.ClusterReportChangeRequest{}
	} else {
		report = &kyvernov1alpha2.ReportChangeRequest{}
	}
	report.SetName(name)
	report.SetNamespace(name)
	SetManagedByKyvernoLabel(report)
	return report
}

func CreateReport(report kyvernov1alpha2.ReportChangeRequestInterface, client versioned.Interface) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	switch v := report.(type) {
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

func UpdateReport(report kyvernov1alpha2.ReportChangeRequestInterface, client versioned.Interface) (kyvernov1alpha2.ReportChangeRequestInterface, error) {
	switch v := report.(type) {
	case *kyvernov1alpha2.ReportChangeRequest:
		report, err := client.KyvernoV1alpha2().ReportChangeRequests(report.GetNamespace()).Update(context.TODO(), v, metav1.UpdateOptions{})
		return report, err
	case *kyvernov1alpha2.ClusterReportChangeRequest:
		report, err := client.KyvernoV1alpha2().ClusterReportChangeRequests().Update(context.TODO(), v, metav1.UpdateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
