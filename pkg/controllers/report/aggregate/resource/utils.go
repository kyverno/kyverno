package resource

import (
	"context"
	"errors"

	kyvernov1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	"github.com/kyverno/kyverno/pkg/report"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func mergeReports(policyMap map[string]policyMapEntry, vapMap sets.Set[string], accumulator map[string]policyreportv1alpha2.PolicyReportResult, uid types.UID, reports ...kyvernov1alpha2.ReportInterface) {
	for _, report := range reports {
		if report != nil {
			for _, result := range report.GetResults() {
				if result.Source == "ValidatingAdmissionPolicy" {
					if vapMap != nil && vapMap.Has(result.Policy) {
						key := result.Source + "/" + result.Policy + "/" + string(uid)
						if rule, exists := accumulator[key]; !exists {
							accumulator[key] = result
						} else if rule.Timestamp.Seconds < result.Timestamp.Seconds {
							accumulator[key] = result
						}
					}
				} else {
					currentPolicy := policyMap[result.Policy]
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
}

func deleteReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, reportManager report.Interface) error {
	if !controllerutils.IsManagedByKyverno(report) {
		return errors.New("can't delete report because it is not managed by kyverno")
	}
	return reportManager.DeleteReport(ctx, report)
}

func updateReport(ctx context.Context, report kyvernov1alpha2.ReportInterface, reportManager report.Interface) (kyvernov1alpha2.ReportInterface, error) {
	if !controllerutils.IsManagedByKyverno(report) {
		return nil, errors.New("can't update report because it is not managed by kyverno")
	}
	return reportManager.UpdateReport(ctx, report)
}
