package deleting

import (
	"context"
	"fmt"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
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

// reportName returns the deterministic PolicyReport / ClusterPolicyReport name
// for a given DeletingPolicy or NamespacedDeletingPolicy.
// Format: "dpol-<policy-name>"
func reportName(policy v1beta1.DeletingPolicyLike) string {
	return "dpol-" + policy.GetName()
}

// reportWriter persists per-policy deletion audit results as PolicyReport /
// ClusterPolicyReport objects.  Each call to Write replaces the results of the
// previous execution so that consumers always see the latest run.
type reportWriter struct {
	kyvernoClient versioned.Interface
}

// deletionRecord pairs a resource with the error (nil == success) from its
// deletion attempt.
type deletionRecord struct {
	resource unstructured.Unstructured
	err      error
}

// Write creates or replaces the (Cluster)PolicyReport for policy with the
// provided deletion records.  It is safe to call with an empty records slice
// (the report will be created / updated with zero results).
func (w *reportWriter) Write(ctx context.Context, policy v1beta1.DeletingPolicyLike, records []deletionRecord) {
	results := make([]openreportsv1alpha1.ReportResult, 0, len(records))
	policyName := policy.GetName()
	for _, rec := range records {
		results = append(results, reportutils.DeletingPolicyToReportResult(policyName, rec.resource, rec.err))
	}

	namespace := policy.GetNamespace()
	name := reportName(policy)

	if namespace == "" {
		w.writeClusterReport(ctx, policy, name, results)
	} else {
		w.writeNamespacedReport(ctx, policy, namespace, name, results)
	}
}

// writeClusterReport creates or updates a ClusterPolicyReport for a cluster-scoped DeletingPolicy.
func (w *reportWriter) writeClusterReport(ctx context.Context, policy v1beta1.DeletingPolicyLike, name string, results []openreportsv1alpha1.ReportResult) {
	client := w.kyvernoClient.Wgpolicyk8sV1alpha2().ClusterPolicyReports()

	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get ClusterPolicyReport", "name", name)
		return
	}

	summary := reportutils.CalculateSummary(results)
	wgResults := toWGResults(results)

	if apierrors.IsNotFound(err) {
		report := &policyreportv1alpha2.ClusterPolicyReport{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":  "kyverno",
					"policies.kyverno.io/dpol.name": policy.GetName(),
				},
				Annotations: map[string]string{
					"policies.kyverno.io/source": reportutils.SourceDeletingPolicy,
				},
			},
			Scope: &corev1.ObjectReference{
				APIVersion: fmt.Sprintf("%s/%s", v1beta1.GroupVersion.Group, v1beta1.GroupVersion.Version),
				Kind:       policy.GetKind(),
				Name:       policy.GetName(),
				UID:        types.UID(policy.GetUID()),
			},
			Results: wgResults,
			Summary: toWGSummary(summary),
		}
		if _, createErr := client.Create(ctx, report, metav1.CreateOptions{}); createErr != nil {
			logger.Error(createErr, "failed to create ClusterPolicyReport", "name", name)
		}
		return
	}

	updated := existing.DeepCopy()
	updated.Results = wgResults
	updated.Summary = toWGSummary(summary)
	if _, updateErr := client.Update(ctx, updated, metav1.UpdateOptions{}); updateErr != nil {
		logger.Error(updateErr, "failed to update ClusterPolicyReport", "name", name)
	}
}

// writeNamespacedReport creates or updates a PolicyReport for a namespaced DeletingPolicy.
func (w *reportWriter) writeNamespacedReport(ctx context.Context, policy v1beta1.DeletingPolicyLike, namespace, name string, results []openreportsv1alpha1.ReportResult) {
	client := w.kyvernoClient.Wgpolicyk8sV1alpha2().PolicyReports(namespace)

	existing, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get PolicyReport", "namespace", namespace, "name", name)
		return
	}

	summary := reportutils.CalculateSummary(results)
	wgResults := toWGResults(results)

	if apierrors.IsNotFound(err) {
		report := &policyreportv1alpha2.PolicyReport{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":   "kyverno",
					"policies.kyverno.io/ndpol.name": policy.GetName(),
				},
				Annotations: map[string]string{
					"policies.kyverno.io/source": reportutils.SourceDeletingPolicy,
				},
			},
			Scope: &corev1.ObjectReference{
				APIVersion: fmt.Sprintf("%s/%s", v1beta1.GroupVersion.Group, v1beta1.GroupVersion.Version),
				Kind:       policy.GetKind(),
				Name:       policy.GetName(),
				Namespace:  namespace,
				UID:        types.UID(policy.GetUID()),
			},
			Results: wgResults,
			Summary: toWGSummary(summary),
		}
		if _, createErr := client.Create(ctx, report, metav1.CreateOptions{}); createErr != nil {
			logger.Error(createErr, "failed to create PolicyReport", "namespace", namespace, "name", name)
		}
		return
	}

	updated := existing.DeepCopy()
	updated.Results = wgResults
	updated.Summary = toWGSummary(summary)
	if _, updateErr := client.Update(ctx, updated, metav1.UpdateOptions{}); updateErr != nil {
		logger.Error(updateErr, "failed to update PolicyReport", "namespace", namespace, "name", name)
	}
}

// toWGResults converts openreports results to the wgpolicy PolicyReportResult type.
func toWGResults(results []openreportsv1alpha1.ReportResult) []policyreportv1alpha2.PolicyReportResult {
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

// toWGSummary converts an openreports summary to the wgpolicy type.
func toWGSummary(s openreportsv1alpha1.ReportSummary) policyreportv1alpha2.PolicyReportSummary {
	return policyreportv1alpha2.PolicyReportSummary{
		Pass:  s.Pass,
		Fail:  s.Fail,
		Warn:  s.Warn,
		Skip:  s.Skip,
		Error: s.Error,
	}
}
