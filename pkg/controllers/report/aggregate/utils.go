package aggregate

import (
	"context"
	"errors"
	"time"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	openreportsclient "github.com/openreports/reports-api/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Maps holds the set of currently-active policy keys per policy type, used to
// filter report results during aggregation (a result whose policy is no longer
// active is dropped). Fields are exported so callers outside this package (the
// integration test framework) can build a Maps and run the real merge via
// MergeReports / AggregateResults.
type Maps struct {
	Pol    map[string]PolicyMapEntry
	Vap    sets.Set[string]
	Mappol sets.Set[string]
	Vpol   sets.Set[string]
	Ivpol  sets.Set[string]
	Gpol   sets.Set[string]
	Mpol   sets.Set[string]
}

// MergeReports merges the results of the given reports into accumulator, keyed
// by (source, policy, [rule], uid). On a key collision it keeps the result with
// the later Timestamp.Seconds (and the first-seen result when timestamps are
// equal), and it drops results whose policy is not present in maps. Pure: no
// I/O, no clients, no clock.
func MergeReports(maps Maps, accumulator map[string]openreportsv1alpha1.ReportResult, uid types.UID, reports ...reportsv1.ReportInterface) {
	for _, report := range reports {
		if report == nil {
			continue
		}
		for _, result := range report.GetResults() {
			switch result.Source {
			case reportutils.SourceValidatingPolicy:
				if maps.Vpol != nil && maps.Vpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceImageValidatingPolicy:
				if maps.Ivpol != nil && maps.Ivpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceGeneratingPolicy:
				if maps.Gpol != nil && maps.Gpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceValidatingAdmissionPolicy:
				if maps.Vap != nil && maps.Vap.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceMutatingAdmissionPolicy:
				if maps.Mappol != nil && maps.Mappol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceMutatingPolicy:
				if maps.Mpol != nil && maps.Mpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			default:
				currentPolicy := maps.Pol[result.Policy]
				if currentPolicy.Rules != nil && currentPolicy.Rules.Has(result.Rule) {
					key := result.Source + "/" + result.Policy + "/" + result.Rule + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			}
		}
	}
}

// AggregateResults runs MergeReports over the given reports and returns the
// merged results as a slice. This is the single-pass, stateless aggregation the
// reports controller performs inside backReconcile, exposed for reuse (for
// example by the integration test framework) without the controller's queues,
// caches, or timers.
func AggregateResults(maps Maps, uid types.UID, reports ...reportsv1.ReportInterface) []openreportsv1alpha1.ReportResult {
	accumulator := map[string]openreportsv1alpha1.ReportResult{}
	MergeReports(maps, accumulator, uid, reports...)
	results := make([]openreportsv1alpha1.ReportResult, 0, len(accumulator))
	for _, result := range accumulator {
		results = append(results, result)
	}
	return results
}

func deleteReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface, orClient openreportsclient.OpenreportsV1alpha1Interface) error {
	if !controllerutils.IsManagedByKyverno(report) {
		return errors.New("can't delete report because it is not managed by kyverno")
	}
	return reportutils.DeleteReport(ctx, report, client, orClient)
}

func updateReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface, orClient openreportsclient.OpenreportsV1alpha1Interface) (reportsv1.ReportInterface, error) {
	if !controllerutils.IsManagedByKyverno(report) {
		return nil, errors.New("can't update report because it is not managed by kyverno")
	}
	return reportutils.UpdateReport(ctx, report, client, orClient)
}

func isTooOld(reportMeta *metav1.PartialObjectMetadata) bool {
	return reportMeta.GetCreationTimestamp().Add(deletionGrace).Before(time.Now())
}
