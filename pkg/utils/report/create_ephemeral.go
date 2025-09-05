package report

import (
	"context"
	"errors"
	"strings"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
