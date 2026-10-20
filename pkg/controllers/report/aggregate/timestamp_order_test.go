package aggregate_test

import (
	"testing"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	"github.com/kyverno/kyverno/pkg/openreports"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func timestampTestMaps() aggregate.Maps {
	const policy = "require-label"
	return aggregate.Maps{
		Pol: map[string]aggregate.PolicyMapEntry{
			policy: {Rules: sets.New[string]("check-label")},
		},
		Vap:    sets.New[string](policy),
		Mappol: sets.New[string](policy),
		Vpol:   sets.New[string](policy),
		Ivpol:  sets.New[string](policy),
		Gpol:   sets.New[string](policy),
		Mpol:   sets.New[string](policy),
	}
}

func ephemeralTimestampResult(result openreportsv1alpha1.ReportResult) *reportsv1.EphemeralReport {
	return &reportsv1.EphemeralReport{
		Spec: reportsv1.EphemeralReportSpec{
			Results: []openreportsv1alpha1.ReportResult{result},
		},
	}
}

func TestAggregateResultsOrdersNanosecondsAcrossPolicySources(t *testing.T) {
	const second int64 = 1700000000
	sources := []string{
		reportutils.SourceKyverno,
		reportutils.SourceValidatingPolicy,
		reportutils.SourceImageValidatingPolicy,
		reportutils.SourceGeneratingPolicy,
		reportutils.SourceValidatingAdmissionPolicy,
		reportutils.SourceMutatingAdmissionPolicy,
		reportutils.SourceMutatingPolicy,
	}
	transitions := []struct {
		name          string
		before, after openreportsv1alpha1.Result
	}{
		{"fail-to-pass", openreports.StatusFail, openreports.StatusPass},
		{"pass-to-fail", openreports.StatusPass, openreports.StatusFail},
	}
	for _, source := range sources {
		for _, transition := range transitions {
			old := openreportsv1alpha1.ReportResult{
				Source: source, Policy: "require-label", Rule: "check-label",
				Result: transition.before,
				Timestamp: metav1.Timestamp{Seconds: second, Nanos: 100_000_000},
			}
			newer := old
			newer.Result = transition.after
			newer.Timestamp.Nanos = 900_000_000
			orders := []struct {
				name string
				first, second openreportsv1alpha1.ReportResult
			}{
				{"older-first", old, newer},
				{"newer-first", newer, old},
			}
			for _, order := range orders {
				t.Run(source+"/"+transition.name+"/"+order.name, func(t *testing.T) {
					results := aggregate.AggregateResults(timestampTestMaps(), types.UID("uid"),
						ephemeralTimestampResult(order.first), ephemeralTimestampResult(order.second))
					require.Len(t, results, 1)
					require.Equal(t, newer.Result, results[0].Result)
					require.Equal(t, newer.Timestamp, results[0].Timestamp)
				})
			}
		}
	}
}

func TestAggregateResultsTimestampCompatibility(t *testing.T) {
	const second int64 = 1700000000
	tests := []struct {
		name string
		first, second metav1.Timestamp
		want openreportsv1alpha1.Result
	}{
		{
			name: "legacy-exact-tie-keeps-first",
			first: metav1.Timestamp{Seconds: second},
			second: metav1.Timestamp{Seconds: second},
			want: openreports.StatusFail,
		},
		{
			name: "nonzero-nanos-exact-tie-keeps-first",
			first: metav1.Timestamp{Seconds: second, Nanos: 200_000_000},
			second: metav1.Timestamp{Seconds: second, Nanos: 200_000_000},
			want: openreports.StatusFail,
		},
		{
			name: "mixed-precision-uses-recorded-order",
			first: metav1.Timestamp{Seconds: second},
			second: metav1.Timestamp{Seconds: second, Nanos: 500_000_000},
			want: openreports.StatusPass,
		},
		{
			name: "mixed-precision-reversed",
			first: metav1.Timestamp{Seconds: second, Nanos: 500_000_000},
			second: metav1.Timestamp{Seconds: second},
			want: openreports.StatusFail,
		},
		{
			name: "strictly-newer-second",
			first: metav1.Timestamp{Seconds: second, Nanos: 900_000_000},
			second: metav1.Timestamp{Seconds: second + 1},
			want: openreports.StatusPass,
		},
		{
			name: "older-second-not-newer-despite-nanos",
			first: metav1.Timestamp{Seconds: second + 1},
			second: metav1.Timestamp{Seconds: second, Nanos: 900_000_000},
			want: openreports.StatusFail,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			old := openreportsv1alpha1.ReportResult{
				Source: reportutils.SourceKyverno, Policy: "require-label",
				Rule: "check-label", Result: openreports.StatusFail, Timestamp: tc.first,
			}
			newer := old
			newer.Result = openreports.StatusPass
			newer.Timestamp = tc.second
			results := aggregate.AggregateResults(timestampTestMaps(), types.UID("uid"),
				ephemeralTimestampResult(old), ephemeralTimestampResult(newer))
			require.Len(t, results, 1)
			require.Equal(t, tc.want, results[0].Result)
		})
	}
}

func TestPermanentReportUpdatedByNewerEphemeral(t *testing.T) {
	old := openreportsv1alpha1.ReportResult{
		Source: reportutils.SourceKyverno, Policy: "require-label", Rule: "check-label",
		Result: openreports.StatusFail,
		Timestamp: metav1.Timestamp{Seconds: 1700000000, Nanos: 100_000_000},
	}
	newer := old
	newer.Result = openreports.StatusPass
	newer.Timestamp.Nanos = 900_000_000
	permanent := &openreports.ReportAdapter{
		Report: &openreportsv1alpha1.Report{
			Results: []openreportsv1alpha1.ReportResult{old},
		},
	}
	results := aggregate.AggregateResults(timestampTestMaps(), types.UID("uid"),
		permanent, ephemeralTimestampResult(newer))
	require.Len(t, results, 1)
	require.Equal(t, openreports.StatusPass, results[0].Result)
	require.Equal(t, newer.Timestamp, results[0].Timestamp)
}

func TestMergedResultAndSummaryAcrossReportAdapters(t *testing.T) {
	old := openreportsv1alpha1.ReportResult{
		Source: reportutils.SourceKyverno, Policy: "require-label", Rule: "check-label",
		Result: openreports.StatusFail,
		Timestamp: metav1.Timestamp{Seconds: 1700000000, Nanos: 100_000_000},
	}
	newer := old
	newer.Result = openreports.StatusPass
	newer.Timestamp.Nanos = 900_000_000

	results := aggregate.AggregateResults(timestampTestMaps(), types.UID("uid"),
		ephemeralTimestampResult(old), ephemeralTimestampResult(newer))

	tests := []struct {
		name string
		makeReport func() reportsv1.ReportInterface
		summary func(reportsv1.ReportInterface) (int, int)
	}{
		{
			name: "openreports-namespaced",
			makeReport: func() reportsv1.ReportInterface {
				return &openreports.ReportAdapter{Report: &openreportsv1alpha1.Report{}}
			},
			summary: func(r reportsv1.ReportInterface) (int, int) {
				s := r.(*openreports.ReportAdapter).Report.Summary
				return int(s.Pass), int(s.Fail)
			},
		},
		{
			name: "openreports-cluster",
			makeReport: func() reportsv1.ReportInterface {
				return &openreports.ClusterReportAdapter{ClusterReport: &openreportsv1alpha1.ClusterReport{}}
			},
			summary: func(r reportsv1.ReportInterface) (int, int) {
				s := r.(*openreports.ClusterReportAdapter).ClusterReport.Summary
				return int(s.Pass), int(s.Fail)
			},
		},
		{
			name: "wgpolicy-namespaced",
			makeReport: func() reportsv1.ReportInterface {
				return openreports.NewWGPolAdapter(&policyreportv1alpha2.PolicyReport{})
			},
			summary: func(r reportsv1.ReportInterface) (int, int) {
				s := r.(*openreports.WgpolicyReportAdapter).PolicyReport.Summary
				return int(s.Pass), int(s.Fail)
			},
		},
		{
			name: "wgpolicy-cluster",
			makeReport: func() reportsv1.ReportInterface {
				return openreports.NewWGCpolAdapter(&policyreportv1alpha2.ClusterPolicyReport{})
			},
			summary: func(r reportsv1.ReportInterface) (int, int) {
				s := r.(*openreports.WgpolicyClusterReportAdapter).ClusterPolicyReport.Summary
				return int(s.Pass), int(s.Fail)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := tc.makeReport()
			reportutils.SetResults(report, results...)
			got := report.GetResults()
			require.Len(t, got, 1)
			require.Equal(t, openreports.StatusPass, got[0].Result)
			require.Equal(t, newer.Timestamp, got[0].Timestamp)
			pass, fail := tc.summary(report)
			require.Equal(t, 1, pass)
			require.Zero(t, fail)
		})
	}
}
