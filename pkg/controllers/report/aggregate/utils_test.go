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

// TestMergeReports_AggregatesLegacyKyvernoSource proves reporting continuity
// for #17491: a legacy kyverno.io (ClusterPolicy/Policy) report result, whose
// ReportResult.Source is reportutils.SourceKyverno, is still aggregated by
// MergeReports exactly as it was before the explicit
// `case reportutils.SourceKyverno: fallthrough` was added to the switch in
// utils.go, keyed the same way as the (still-existing) default branch.
func TestMergeReports_AggregatesLegacyKyvernoSource(t *testing.T) {
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
	maps := aggregate.Maps{
		Pol: map[string]aggregate.PolicyMapEntry{
			"default/require-app-label": {
				Rules: sets.New[string]("check-app-label"),
			},
		},
	}

	accumulator := map[string]openreportsv1alpha1.ReportResult{}
	aggregate.MergeReports(maps, accumulator, uid, report)

	key := reportutils.SourceKyverno + "/default/require-app-label/check-app-label/" + uid
	result, ok := accumulator[key]
	assert.True(t, ok, "expected legacy kyverno.io result to be aggregated under key %q, got keys %v", key, keys(accumulator))
	assert.Equal(t, reportutils.SourceKyverno, result.Source)
	assert.Equal(t, "check-app-label", result.Rule)
}

// TestMergeReports_LegacyAndCELCoexist guards against the new explicit
// `case reportutils.SourceKyverno:` accidentally swallowing or altering
// results from other (CEL/native) sources: a legacy result and a
// SourceValidatingPolicy result in the same report must both land in the
// accumulator.
func TestMergeReports_LegacyAndCELCoexist(t *testing.T) {
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
		Pol: map[string]aggregate.PolicyMapEntry{
			"default/require-app-label": {
				Rules: sets.New[string]("check-app-label"),
			},
		},
		Vpol: sets.New[string]("check-deployment-replicas"),
	}

	accumulator := map[string]openreportsv1alpha1.ReportResult{}
	aggregate.MergeReports(maps, accumulator, uid, report)

	legacyKey := reportutils.SourceKyverno + "/default/require-app-label/check-app-label/" + uid
	celKey := reportutils.SourceValidatingPolicy + "/check-deployment-replicas/" + uid

	_, legacyOK := accumulator[legacyKey]
	_, celOK := accumulator[celKey]
	assert.True(t, legacyOK, "expected legacy kyverno.io result under key %q, got keys %v", legacyKey, keys(accumulator))
	assert.True(t, celOK, "expected CEL ValidatingPolicy result under key %q, got keys %v", celKey, keys(accumulator))
	assert.Len(t, accumulator, 2)
}

func keys(m map[string]openreportsv1alpha1.ReportResult) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
