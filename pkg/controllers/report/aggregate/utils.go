package aggregate

import (
	"context"
	"errors"
	"time"

	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type maps struct {
	pol   map[string]policyMapEntry
	vap   sets.Set[string]
	vpol  sets.Set[string]
	ivpol sets.Set[string]
}

func mergeReports(maps maps, accumulator map[string]policyreportv1alpha2.PolicyReportResult, uid types.UID, reports ...reportsv1.ReportInterface) {
	for _, report := range reports {
		if report == nil {
			continue
		}
		for _, result := range report.GetResults() {
			switch result.Source {
			case reportutils.SourceValidatingPolicy:
				if maps.vpol != nil && maps.vpol.Has(result.Policy) {
					key := result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if time.Unix(rule.Timestamp.Seconds, int64(rule.Timestamp.Nanos)).Before(time.Unix(result.Timestamp.Seconds, int64(result.Timestamp.Nanos))) {
						accumulator[key] = result
					}
				}
			case reportutils.SourceImageVerificationPolicy:
				if maps.ivpol != nil && maps.ivpol.Has(result.Policy) {
					key := result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if time.Unix(rule.Timestamp.Seconds, int64(rule.Timestamp.Nanos)).Before(time.Unix(result.Timestamp.Seconds, int64(result.Timestamp.Nanos))) {
						accumulator[key] = result
					}
				}
			case reportutils.SourceValidatingAdmissionPolicy:
				if maps.vap != nil && maps.vap.Has(result.Policy) {
					key := result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if time.Unix(rule.Timestamp.Seconds, int64(rule.Timestamp.Nanos)).Before(time.Unix(result.Timestamp.Seconds, int64(result.Timestamp.Nanos))) {
						accumulator[key] = result
					}
				}
			default:
				currentPolicy := maps.pol[result.Policy]
				if currentPolicy.rules != nil && currentPolicy.rules.Has(result.Rule) {
					key := result.Source + "/" + result.Policy + "/" + result.Rule + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if time.Unix(rule.Timestamp.Seconds, int64(rule.Timestamp.Nanos)).Before(time.Unix(result.Timestamp.Seconds, int64(result.Timestamp.Nanos))) {
						accumulator[key] = result
					}
				}
			}
		}
	}
}

func deleteReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) error {
	if !controllerutils.IsManagedByKyverno(report) {
		return errors.New("can't delete report because it is not managed by kyverno")
	}
	return reportutils.DeleteReport(ctx, report, client)
}

func updateReport(ctx context.Context, report reportsv1.ReportInterface, client versioned.Interface) (reportsv1.ReportInterface, error) {
	if !controllerutils.IsManagedByKyverno(report) {
		return nil, errors.New("can't update report because it is not managed by kyverno")
	}
	return reportutils.UpdateReport(ctx, report, client)
}

func isTooOld(reportMeta *metav1.PartialObjectMetadata) bool {
	return reportMeta.GetCreationTimestamp().Add(deletionGrace).Before(time.Now())
}
