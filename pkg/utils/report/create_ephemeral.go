package report

import (
	"context"
	"errors"
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsPolicyReportable(pol interface{}) bool {
	switch p := pol.(type) {
	case kyvernov1.PolicyInterface:
		labels := p.GetLabels()
		if _, ok := labels[kyverno.LabelExcludeReporting]; ok {
			return false
		}
	case metav1.Object:
		labels := p.GetLabels()
		if _, ok := labels[kyverno.LabelExcludeReporting]; ok {
			return false
		}
	}
	return true
}

// IsNamespaceTerminationError checks if the error is due to namespace being terminated
func IsNamespaceTerminationError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a forbidden error from Kubernetes API
	if !apierrors.IsForbidden(err) {
		return false
	}

	// Check if the error message indicates namespace termination (case-insensitive)
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "unable to create new content in namespace") &&
		strings.Contains(errMsg, "because it is being terminated")
}

func CreateEphemeralReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) (reportsv1.ReportInterface, error) {
	switch v := report.(type) {
	case *reportsv1.EphemeralReport:
		report, err := client.ReportsV1().EphemeralReports(report.GetNamespace()).Create(ctx, v, metav1.CreateOptions{})
		return report, err
	case *reportsv1.ClusterEphemeralReport:
		report, err := client.ReportsV1().ClusterEphemeralReports().Create(ctx, v, metav1.CreateOptions{})
		return report, err
	default:
		return nil, errors.New("unknow type")
	}
}
