package report

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var ReportingCfg ReportingConfiguration

type reportingConfig struct {
	helper            sets.Set[string]
	allowedRuleStatus map[engineapi.RuleStatus]struct{}
}

func (r *reportingConfig) ValidateReportsEnabled() bool {
	return r.helper.Has("validate")
}

func (r *reportingConfig) MutateReportsEnabled() bool {
	return r.helper.Has("mutate")
}

func (r *reportingConfig) MutateExistingReportsEnabled() bool {
	return r.helper.Has("mutateExisting")
}

func (r *reportingConfig) ImageVerificationReportsEnabled() bool {
	return r.helper.Has("imageVerify")
}

func (r *reportingConfig) GenerateReportsEnabled() bool {
	return r.helper.Has("generate")
}

func (r *reportingConfig) IsStatusAllowed(s engineapi.RuleStatus) bool {
	if r == nil {
		return true
	}
	_, exists := r.allowedRuleStatus[s]
	return exists
}

func (r *reportingConfig) IsResultAllowed(res openreportsv1alpha1.Result) bool {
	if r == nil {
		return true
	}
	switch res {
	case openreports.StatusPass:
		return r.IsStatusAllowed(engineapi.RuleStatusPass)
	case openreports.StatusFail:
		return r.IsStatusAllowed(engineapi.RuleStatusFail)
	case openreports.StatusWarn:
		return r.IsStatusAllowed(engineapi.RuleStatusWarn)
	case openreports.StatusError:
		return r.IsStatusAllowed(engineapi.RuleStatusError)
	case openreports.StatusSkip:
		return r.IsStatusAllowed(engineapi.RuleStatusSkip)
	default:
		return r.IsStatusAllowed(engineapi.RuleStatus(res))
	}
}

func IsStatusAllowed(s engineapi.RuleStatus) bool {
	if ReportingCfg == nil {
		return true
	}
	return ReportingCfg.IsStatusAllowed(s)
}

func IsResultAllowed(res openreportsv1alpha1.Result) bool {
	if ReportingCfg == nil {
		return true
	}
	return ReportingCfg.IsResultAllowed(res)
}

func newReportingConfig(allowedRuleStatus []string, items ...string) ReportingConfiguration {
	allowedStatusMap := make(map[engineapi.RuleStatus]struct{})
	for _, status := range allowedRuleStatus {
		switch status {
		case "pass":
			allowedStatusMap[engineapi.RuleStatusPass] = struct{}{}
		case "fail":
			allowedStatusMap[engineapi.RuleStatusFail] = struct{}{}
		case "warn":
			allowedStatusMap[engineapi.RuleStatusWarn] = struct{}{}
		case "error":
			allowedStatusMap[engineapi.RuleStatusError] = struct{}{}
		case "skip":
			allowedStatusMap[engineapi.RuleStatusSkip] = struct{}{}
		}
	}
	return &reportingConfig{
		helper:            sets.New(items...),
		allowedRuleStatus: allowedStatusMap,
	}
}

func NewReportingConfig(allowedRuleStatus []string, items ...string) ReportingConfiguration {
	if ReportingCfg != nil {
		return ReportingCfg
	}
	ReportingCfg = newReportingConfig(allowedRuleStatus, items...)
	return ReportingCfg
}

func ResetReportingConfig(allowedRuleStatus []string, items ...string) ReportingConfiguration {
	ReportingCfg = newReportingConfig(allowedRuleStatus, items...)
	return ReportingCfg
}

type ReportingConfiguration interface {
	ValidateReportsEnabled() bool
	MutateReportsEnabled() bool
	MutateExistingReportsEnabled() bool
	ImageVerificationReportsEnabled() bool
	GenerateReportsEnabled() bool
	IsStatusAllowed(engineapi.RuleStatus) bool
	IsResultAllowed(openreportsv1alpha1.Result) bool
}
