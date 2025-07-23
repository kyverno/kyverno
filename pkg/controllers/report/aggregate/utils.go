package aggregate

import (
	"context"
	"errors"
	"time"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
	openreportsclient "openreports.io/pkg/client/clientset/versioned/typed/openreports.io/v1alpha1"
)

type maps struct {
	pol    map[string]policyMapEntry
	vap    sets.Set[string]
	mappol sets.Set[string]
	vpol   sets.Set[string]
	ivpol  sets.Set[string]
	gpol   sets.Set[string]
	mpol   sets.Set[string]
}

func mergeReports(maps maps, accumulator map[string]openreportsv1alpha1.ReportResult, uid types.UID, reports ...reportsv1.ReportInterface) {
	for _, report := range reports {
		if report == nil {
			continue
		}
		for _, result := range report.GetResults() {
			switch result.Source {
			case reportutils.SourceValidatingPolicy:
				if maps.vpol != nil && maps.vpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceImageValidatingPolicy:
				if maps.ivpol != nil && maps.ivpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceGeneratingPolicy:
				if maps.gpol != nil && maps.gpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceValidatingAdmissionPolicy:
				if maps.vap != nil && maps.vap.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceMutatingAdmissionPolicy:
				if maps.mappol != nil && maps.mappol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			case reportutils.SourceMutatingPolicy:
				if maps.mpol != nil && maps.mpol.Has(result.Policy) {
					key := result.Source + "/" + result.Policy + "/" + string(uid)
					if rule, exists := accumulator[key]; !exists {
						accumulator[key] = result
					} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
						accumulator[key] = result
					}
				}
			default:
				currentPolicy := maps.pol[result.Policy]
				if currentPolicy.rules != nil && currentPolicy.rules.Has(result.Rule) {
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
