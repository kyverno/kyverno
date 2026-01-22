package report

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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
	_, exists := r.allowedRuleStatus[s]
	return exists
}

func NewReportingConfig(allowedRuleStatus []string, items ...string) ReportingConfiguration {
	if ReportingCfg != nil {
		return ReportingCfg
	}
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
	ReportingCfg = &reportingConfig{
		helper:            sets.New(items...),
		allowedRuleStatus: allowedStatusMap,
	}
	return ReportingCfg
}

type ReportingConfiguration interface {
	ValidateReportsEnabled() bool
	MutateReportsEnabled() bool
	MutateExistingReportsEnabled() bool
	ImageVerificationReportsEnabled() bool
	GenerateReportsEnabled() bool
	IsStatusAllowed(engineapi.RuleStatus) bool
}
