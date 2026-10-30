package aggregate_test

import (
	"testing"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TestMergeReports_DropsLegacyKyvernoSource proves legacy kyverno.io
// (ClusterPolicy/Policy) report results are no longer aggregated: since this
// controller no longer populates Maps.Pol, a SourceKyverno result falls
// through the default branch and is dropped regardless of Maps' other fields.
func TestMergeReports_DropsLegacyKyvernoSource(t *testing.T) {
	const uid = "test-uid"
	report := &reportsv1.EphemeralReport{
		Spec: reportsv1.EphemeralReportSpec{
			Results: []openreportsv1alpha1.ReportResult{
				{
					Source: reportutils.SourceKyverno,
					Policy: "default/require-app-label",
					Rule:   "check-app-label",
				},
			},
		},
	}

	for _, maps := range []aggregate.Maps{
		{},
		{Vpol: sets.New[string]("x")},
	} {
		accumulator := map[string]openreportsv1alpha1.ReportResult{}
		aggregate.MergeReports(maps, accumulator, uid, report)
		assert.Empty(t, accumulator, "expected legacy kyverno.io result to be dropped, got keys %v", keys(accumulator))
	}
}

// TestMergeReports_LegacyDroppedCELKept proves that, in a report carrying both
// a legacy kyverno.io result and a CEL (ValidatingPolicy) result, the legacy
// result is dropped while the CEL result is kept.
func TestMergeReports_LegacyDroppedCELKept(t *testing.T) {
	const uid = "test-uid"
	report := &reportsv1.EphemeralReport{
		Spec: reportsv1.EphemeralReportSpec{
			Results: []openreportsv1alpha1.ReportResult{
				{
					Source: reportutils.SourceKyverno,
					Policy: "default/require-app-label",
					Rule:   "check-app-label",
				},
				{
					Source: reportutils.SourceValidatingPolicy,
					Policy: "check-deployment-replicas",
				},
			},
		},
	}
	maps := aggregate.Maps{
		Vpol: sets.New[string]("check-deployment-replicas"),
	}

	accumulator := map[string]openreportsv1alpha1.ReportResult{}
	aggregate.MergeReports(maps, accumulator, uid, report)

	celKey := reportutils.SourceValidatingPolicy + "/check-deployment-replicas/" + uid
	_, celOK := accumulator[celKey]
	assert.True(t, celOK, "expected CEL ValidatingPolicy result under key %q, got keys %v", celKey, keys(accumulator))
	assert.Len(t, accumulator, 1)
}

func keys(m map[string]openreportsv1alpha1.ReportResult) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
