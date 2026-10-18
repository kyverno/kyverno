package framework

import (
	"context"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/breaker"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// EnableReporting turns on admission report emission for the current test
// process. Call it BEFORE NewTestEnv / NewTestEnvWithOptions: the reporting
// config is a process-wide singleton that NewTestEnv otherwise locks to
// "disabled" (first caller wins). It enables every report kind and rule status
// so any policy type can be exercised, and installs a non-tripping reports
// breaker, because the admission handler calls breaker.GetReportsBreaker().Do
// which returns nil (and would panic) when the breaker is unset.
func EnableReporting() {
	reportutils.NewReportingConfig(
		[]string{"pass", "fail", "warn", "error", "skip"},
		"validate", "mutate", "mutateExisting", "imageVerify", "generate",
	)
	breaker.SetReportsBreaker(breaker.NewBreaker("reports", nil))
}

// AggregateEphemeralReports reproduces the reports controller's single-pass,
// stateless aggregation for one resource, without running any controller,
// queue, or cache. It lists the EphemeralReports for the given resource UID,
// runs the real production merge (aggregate.AggregateResults) filtered by the
// active policies in m, and returns a PolicyReport built from the merged
// results (summary included). The caller persists and/or asserts on it.
//
// m is generic across policy types, so this serves vpol/mpol/gpol/ivpol alike:
// populate the matching field of aggregate.Maps with the active policy keys
// (cache.MetaObjectToName(policy).String(), the same key the controller uses).
func AggregateEphemeralReports(
	ctx context.Context,
	client kyvernoclient.Interface,
	namespace, reportName string,
	uid types.UID,
	scope *corev1.ObjectReference,
	m aggregate.Maps,
) (reportsv1.ReportInterface, error) {
	selector, err := reportutils.SelectorResourceUidEquals(uid)
	if err != nil {
		return nil, err
	}
	list, err := client.ReportsV1().EphemeralReports(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	reports := make([]reportsv1.ReportInterface, 0, len(list.Items))
	for i := range list.Items {
		reports = append(reports, &list.Items[i])
	}
	results := aggregate.AggregateResults(m, uid, reports...)
	return reportutils.NewPolicyReport(namespace, reportName, scope, false, results...), nil
}
