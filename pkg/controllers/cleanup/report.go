package cleanup

import (
	"context"
	"fmt"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// cleanupReportName returns the deterministic (Cluster)PolicyReport name for a
// given CleanupPolicy or ClusterCleanupPolicy.  Format: "cleanup-<policy-name>"
func cleanupReportName(policy kyvernov2.CleanupPolicyInterface) string {
	return "cleanup-" + policy.GetName()
}

// cleanupDeletionRecord pairs a resource with the error (nil == success) from
// its deletion attempt.
type cleanupDeletionRecord struct {
	resource unstructured.Unstructured
	err      error
}

// cleanupReportWriter persists per-policy deletion audit results as
// PolicyReport / ClusterPolicyReport objects.  Each Write call replaces the
// results of the previous execution.
type cleanupReportWriter struct {
	kyvernoClient versioned.Interface
}

// Write creates or replaces the (Cluster)PolicyReport for policy.
func (w *cleanupReportWriter) Write(ctx context.Context, policy kyvernov2.CleanupPolicyInterface, records []cleanupDeletionRecord) {
	results := make([]openreportsv1alpha1.ReportResult, 0, len(records))
	policyName := policy.GetName()
	for _, rec := range records {
		results = append(results, reportutils.CleanupPolicyToReportResult(policyName, rec.resource, rec.err))
	}

	namespace := policy.GetNamespace()
	name := cleanupReportName(policy)

	if namespace == "" {
		w.writeClusterReport(ctx, policy, name, results)
	} else {
		w.writeNamespacedReport(ctx, policy, namespace, name, results)
	}
}

func (w *cleanupReportWriter) writeClusterReport(ctx context.Context, policy kyvernov2.CleanupPolicyInterface, name string, results []openreportsv1alpha1.ReportResult) {
	client := w.kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports()

	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get ClusterPolicyReport", "name", name)
		return
	}

	summary := reportutils.CalculateSummary(results)
	wgResults := cleanupToWGResults(results)

	if apierrors.IsNotFound(err) {
		report := &policyreportv1alpha2.ClusterPolicyReport{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":   "kyverno",
					"kyverno.io/cleanup-policy.name": policy.GetName(),
				},
				Annotations: map[string]string{
					"policies.kyverno.io/source": reportutils.SourceCleanupPolicy,
				},
			},
			Scope: &corev1.ObjectReference{
				APIVersion: fmt.Sprintf("%s/%s", "kyverno.io", "v2"),
				Kind:       policy.GetKind(),
				Name:       policy.GetName(),
				UID:        types.UID(policy.GetUID()),
			},
			Results: wgResults,
			Summary: cleanupToWGSummary(summary),
		}
		if _, createErr := client.Create(ctx, report, metav1.CreateOptions{}); createErr != nil {
			logger.Error(createErr, "failed to create ClusterPolicyReport", "name", name)
		}
		return
	}

	updated := existing.DeepCopy()
	updated.Results = wgResults
	updated.Summary = cleanupToWGSummary(summary)
	if _, updateErr := client.Update(ctx, updated, metav1.UpdateOptions{}); updateErr != nil {
		logger.Error(updateErr, "failed to update ClusterPolicyReport", "name", name)
	}
}

func (w *cleanupReportWriter) writeNamespacedReport(ctx context.Context, policy kyvernov2.CleanupPolicyInterface, namespace, name string, results []openreportsv1alpha1.ReportResult) {
	client := w.kyvernoClient.Wgpolicyk8sV1alpha2().PolicyReports(namespace)

	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get PolicyReport", "namespace", namespace, "name", name)
		return
	}

	summary := reportutils.CalculateSummary(results)
	wgResults := cleanupToWGResults(results)

	if apierrors.IsNotFound(err) {
		report := &policyreportv1alpha2.PolicyReport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":   "kyverno",
					"kyverno.io/cleanup-policy.name": policy.GetName(),
				},
				Annotations: map[string]string{
					"policies.kyverno.io/source": reportutils.SourceCleanupPolicy,
				},
			},
			Scope: &corev1.ObjectReference{
				APIVersion: fmt.Sprintf("%s/%s", "kyverno.io", "v2"),
				Kind:       policy.GetKind(),
				Name:       policy.GetName(),
				Namespace:  namespace,
				UID:        types.UID(policy.GetUID()),
			},
			Results: wgResults,
			Summary: cleanupToWGSummary(summary),
		}
		if _, createErr := client.Create(ctx, report, metav1.CreateOptions{}); createErr != nil {
			logger.Error(createErr, "failed to create PolicyReport", "namespace", namespace, "name", name)
		}
		return
	}

	updated := existing.DeepCopy()
	updated.Results = wgResults
	updated.Summary = cleanupToWGSummary(summary)
	if _, updateErr := client.Update(ctx, updated, metav1.UpdateOptions{}); updateErr != nil {
		logger.Error(updateErr, "failed to update PolicyReport", "namespace", namespace, "name", name)
	}
}

func cleanupToWGResults(results []openreportsv1alpha1.ReportResult) []policyreportv1alpha2.PolicyReportResult {
	out := make([]policyreportv1alpha2.PolicyReportResult, 0, len(results))
	for _, r := range results {
		out = append(out, policyreportv1alpha2.PolicyReportResult{
			Source:           r.Source,
			Policy:           r.Policy,
			Rule:             r.Rule,
			Resources:        r.Subjects,
			Message:          r.Description,
			Result:           policyreportv1alpha2.PolicyResult(r.Result),
			Scored:           r.Scored,
			Timestamp:        r.Timestamp,
			Properties:       r.Properties,
			Category:         r.Category,
			Severity:         policyreportv1alpha2.PolicySeverity(r.Severity),
			ResourceSelector: r.ResourceSelector,
		})
	}
	return out
}

func cleanupToWGSummary(s openreportsv1alpha1.ReportSummary) policyreportv1alpha2.PolicyReportSummary {
	return policyreportv1alpha2.PolicyReportSummary{
		Pass:  s.Pass,
		Fail:  s.Fail,
		Warn:  s.Warn,
		Skip:  s.Skip,
		Error: s.Error,
	}
}
